package report

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"infra-audit/internal/model"
)

func ApplyDOCXBranding(docxPath string, reportDir string, meta model.ReportMeta) error {
	watermarkPath := filepath.Join(reportDir, "assets", "watermark.png")
	footerBgPath := filepath.Join(reportDir, "assets", "footer-bg.png")

	watermark, err := os.ReadFile(watermarkPath)
	if err != nil {
		return nil
	}

	footerBg, err := os.ReadFile(footerBgPath)
	if err != nil {
		return nil
	}

	docxBytes, err := os.ReadFile(docxPath)
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	if err != nil {
		return err
	}

	files := map[string][]byte{}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return readErr
		}
		files[f.Name] = data
	}

	files["word/media/audit-watermark.png"] = watermark
	files["word/media/audit-footer-bg.png"] = footerBg

	// Footer icons
	iconDir := filepath.Join(reportDir, "assets")
	if iconEmail, err := os.ReadFile(filepath.Join(iconDir, "icon-email.png")); err == nil {
		files["word/media/audit-icon-email.png"] = iconEmail
	}
	if iconWeb, err := os.ReadFile(filepath.Join(iconDir, "icon-web.png")); err == nil {
		files["word/media/audit-icon-web.png"] = iconWeb
	}
	if iconPhone, err := os.ReadFile(filepath.Join(iconDir, "icon-phone.png")); err == nil {
		files["word/media/audit-icon-phone.png"] = iconPhone
	}

	if _, ok := files["word/settings.xml"]; !ok {
		files["word/settings.xml"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat></w:settings>`)
	}

	files["word/header1.xml"] = []byte(auditHeaderXML())
	files["word/footer1.xml"] = []byte(auditFooterXML(meta))

	files["word/_rels/header1.xml.rels"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdWatermark" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/audit-watermark.png"/></Relationships>`)

	files["word/_rels/footer1.xml.rels"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdFooterBg" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/audit-footer-bg.png"/><Relationship Id="rIdIconEmail" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/audit-icon-email.png"/><Relationship Id="rIdIconWeb" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/audit-icon-web.png"/><Relationship Id="rIdIconPhone" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/audit-icon-phone.png"/></Relationships>`)

	files["word/_rels/document.xml.rels"] = []byte(addDocumentRelationships(string(files["word/_rels/document.xml.rels"])))
	files["word/document.xml"] = []byte(addHeaderFooterReferences(string(files["word/document.xml"])))
	files["[Content_Types].xml"] = []byte(addContentTypeOverrides(string(files["[Content_Types].xml"])))

	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := w.Write(files[name]); err != nil {
			_ = zw.Close()
			return err
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}

	tmp := docxPath + ".tmp"
	if err := os.WriteFile(tmp, out.Bytes(), 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmp, docxPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	return nil
}

func addDocumentRelationships(xml string) string {
	if strings.TrimSpace(xml) == "" {
		xml = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`
	}
	xml = addRelationship(xml, "rIdAuditHeader", "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header", "header1.xml")
	xml = addRelationship(xml, "rIdAuditFooter", "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer", "footer1.xml")
	return xml
}

func addRelationship(xml string, id string, relType string, target string) string {
	if strings.Contains(xml, `Id="`+id+`"`) {
		return xml
	}
	rel := fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s"/>`, id, relType, target)
	return strings.Replace(xml, `</Relationships>`, rel+`</Relationships>`, 1)
}

func addHeaderFooterReferences(xml string) string {
	if strings.Contains(xml, "rIdAuditHeader") && strings.Contains(xml, "rIdAuditFooter") {
		return xml
	}
	refs := `<w:headerReference w:type="default" r:id="rIdAuditHeader"/><w:footerReference w:type="default" r:id="rIdAuditFooter"/>`
	sectEnd := strings.LastIndex(xml, "</w:sectPr>")
	if sectEnd != -1 {
		return xml[:sectEnd] + refs + xml[sectEnd:]
	}
	bodyEnd := strings.LastIndex(xml, "</w:body>")
	if bodyEnd != -1 {
		return xml[:bodyEnd] + "<w:sectPr>" + refs + "</w:sectPr>" + xml[bodyEnd:]
	}
	return xml
}

func addContentTypeOverrides(xml string) string {
	if strings.TrimSpace(xml) == "" {
		xml = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`
	}
	if !strings.Contains(xml, `Extension="png"`) {
		xml = strings.Replace(xml, `</Types>`, `<Default Extension="png" ContentType="image/png"/>`+"\n</Types>", 1)
	}
	if !strings.Contains(xml, `/word/header1.xml`) {
		xml = strings.Replace(xml, `</Types>`, `<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>`+"\n</Types>", 1)
	}
	if !strings.Contains(xml, `/word/footer1.xml`) {
		xml = strings.Replace(xml, `</Types>`, `<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>`+"\n</Types>", 1)
	}
	if !strings.Contains(xml, `/word/settings.xml`) {
		xml = strings.Replace(xml, `</Types>`, `<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>`+"\n</Types>", 1)
	}
	return xml
}

func auditHeaderXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <w:p>
    <w:r>
      <w:drawing>
        <wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" relativeHeight="0" behindDoc="1" locked="0" layoutInCell="1" allowOverlap="1">
          <wp:simplePos x="0" y="0"/>
          <wp:positionH relativeFrom="page"><wp:posOffset>914400</wp:posOffset></wp:positionH>
          <wp:positionV relativeFrom="page"><wp:posOffset>1550000</wp:posOffset></wp:positionV>
          <wp:extent cx="5486400" cy="4216400"/>
          <wp:effectExtent l="0" t="0" r="0" b="0"/>
          <wp:wrapNone/>
          <wp:docPr id="1001" name="Audit Watermark"/>
          <wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>
          <a:graphic>
            <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
              <pic:pic>
                <pic:nvPicPr>
                  <pic:cNvPr id="0" name="audit-watermark.png"/>
                  <pic:cNvPicPr/>
                </pic:nvPicPr>
                <pic:blipFill>
                  <a:blip r:embed="rIdWatermark"><a:alphaModFix amt="55000"/></a:blip>
                  <a:stretch><a:fillRect/></a:stretch>
                </pic:blipFill>
                <pic:spPr>
                  <a:xfrm><a:off x="0" y="0"/><a:ext cx="5486400" cy="4216400"/></a:xfrm>
                  <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
                </pic:spPr>
              </pic:pic>
            </a:graphicData>
          </a:graphic>
        </wp:anchor>
      </w:drawing>
    </w:r>
  </w:p>
</w:hdr>`
}

func auditFooterXML(meta model.ReportMeta) string {
	tmpl := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <w:p>
    <w:pPr>
      <w:ind w:left="720" w:right="-720"/>
      <w:spacing w:before="200" w:after="0"/>
      <w:tabs>
        <w:tab w:val="left" w:pos="3200"/>
        <w:tab w:val="left" w:pos="6400"/>
        <w:tab w:val="right" w:pos="9360"/>
      </w:tabs>
    </w:pPr>
    <w:r>
      <w:drawing>
        <wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" relativeHeight="0" behindDoc="1" locked="0" layoutInCell="1" allowOverlap="1">
          <wp:simplePos x="0" y="0"/>
          <wp:positionH relativeFrom="column"><wp:posOffset>-720000</wp:posOffset></wp:positionH>
          <wp:positionV relativeFrom="paragraph"><wp:posOffset>-76200</wp:posOffset></wp:positionV>
          <wp:extent cx="8539163" cy="1089179"/>
          <wp:effectExtent l="0" t="0" r="0" b="0"/>
          <wp:wrapNone/>
          <wp:docPr id="2001" name="Footer Background"/>
          <a:graphic>
            <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
              <pic:pic>
                <pic:nvPicPr><pic:cNvPr id="0" name="audit-footer-bg.png"/><pic:cNvPicPr/></pic:nvPicPr>
                <pic:blipFill><a:blip r:embed="rIdFooterBg"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>
                <pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="8539163" cy="1089179"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>
              </pic:pic>
            </a:graphicData>
          </a:graphic>
        </wp:anchor>
      </w:drawing>
    </w:r>
    <w:r><w:rPr><w:noProof/></w:rPr><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="114300"><wp:extent cx="228600" cy="228600"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:docPr id="2002" name="icon-email"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="0" name="icon-email"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="rIdIconEmail"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="228600" cy="228600"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>
    <w:r><w:rPr><w:sz w:val="24"/><w:color w:val="111827"/></w:rPr><w:t xml:space="preserve"> delivery@infrajump.com</w:t></w:r>
    <w:r><w:rPr><w:sz w:val="24"/></w:rPr><w:tab/></w:r>
    <w:r><w:rPr><w:noProof/></w:rPr><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="114300"><wp:extent cx="228600" cy="228600"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:docPr id="2003" name="icon-web"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="0" name="icon-web"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="rIdIconWeb"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="228600" cy="228600"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>
    <w:r><w:rPr><w:sz w:val="24"/><w:color w:val="111827"/></w:rPr><w:t xml:space="preserve"> infrajump.com</w:t></w:r>
    <w:r><w:rPr><w:sz w:val="24"/></w:rPr><w:tab/></w:r>
    <w:r><w:rPr><w:noProof/></w:rPr><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="114300"><wp:extent cx="285750" cy="285750"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:docPr id="2004" name="icon-phone"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="0" name="icon-phone"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="rIdIconPhone"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="285750" cy="285750"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>
    <w:r><w:rPr><w:sz w:val="24"/><w:color w:val="111827"/></w:rPr><w:t xml:space="preserve"> +1 650 4847938</w:t></w:r>
  </w:p>
  <w:p>
    <w:pPr>
      <w:ind w:left="720" w:right="-720"/>
      <w:spacing w:before="60" w:after="0"/>
    </w:pPr>
    <w:r><w:rPr><w:sz w:val="24"/><w:color w:val="111827"/></w:rPr><w:t xml:space="preserve">Page </w:t></w:r>
    <w:fldChar w:fldCharType="begin"/>
    <w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r>
    <w:fldChar w:fldCharType="separate"/>
    <w:r><w:rPr><w:sz w:val="24"/></w:rPr><w:t>1</w:t></w:r>
    <w:fldChar w:fldCharType="end"/>
  </w:p>
  <w:p>
    <w:pPr><w:jc w:val="center"/><w:spacing w:before="60" w:after="0"/></w:pPr>
    <w:r><w:rPr><w:sz w:val="18"/><w:color w:val="2F3A44"/></w:rPr>
      <w:t>&#xA9; 2026 InfraJump, Inc. All rights reserved.</w:t>
    </w:r>
  </w:p>
</w:ftr>`
	year := time.Now().Year()
	tmpl = strings.ReplaceAll(tmpl, " delivery@infrajump.com", " "+xmlEsc(meta.AuditorEmail))
	tmpl = strings.ReplaceAll(tmpl, " infrajump.com", " "+xmlEsc(meta.AuditorWebsite))
	tmpl = strings.ReplaceAll(tmpl, " +1 650 4847938", " "+xmlEsc(meta.AuditorPhone))
	tmpl = strings.ReplaceAll(tmpl, "&#xA9; 2026 InfraJump, Inc. All rights reserved.",
		fmt.Sprintf("&#xA9; %d %s. All rights reserved.", year, xmlEsc(meta.AuditorOrg)))
	return tmpl
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
