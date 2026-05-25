package report

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func ConvertHTMLToDOCX(htmlPath string, docxPath string) error {
	reportDir := filepath.Dir(htmlPath)
	base := strings.TrimSuffix(filepath.Base(htmlPath), filepath.Ext(htmlPath))
	cleanHTML := filepath.Join(reportDir, base+".word.html")

	if err := writeWordHTML(htmlPath, cleanHTML); err != nil {
		return err
	}
	defer os.Remove(cleanHTML)

	// Google Docs is stricter than LibreOffice. Pandoc creates cleaner DOCX
	// packages for upload/import, so prefer it when available.
	if pandoc, err := exec.LookPath("pandoc"); err == nil {
		args := []string{
			"--from", "html",
			"--to", "docx",
			"--standalone",
			"--resource-path", reportDir,
			"--output", docxPath,
			cleanHTML,
		}

		cmd := exec.Command(pandoc, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Pandoc HTML to DOCX conversion failed: %w\nCommand: %s %s\n%s", err, pandoc, strings.Join(args, " "), string(out))
		}

		return nil
	}

	// Fallback: LibreOffice conversion. This may look closer to HTML, but
	// Google Docs can reject some LibreOffice HTML-imported DOCX files.
	soffice, err := findLibreOffice()
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "infra-audit-lo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	profileDir := filepath.Join(tmpDir, "lo-profile")

	_ = os.MkdirAll(homeDir, 0755)
	_ = os.MkdirAll(profileDir, 0755)

	args := []string{
		"--headless",
		"--nologo",
		"--nofirststartwizard",
		"--infilter=HTML (StarWriter)",
		"--convert-to",
		"docx:Office Open XML Text",
		"--outdir",
		tmpDir,
		cleanHTML,
	}

	cmd := exec.Command(soffice, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"UserInstallation=file://"+profileDir,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("LibreOffice HTML to DOCX conversion failed: %w\nCommand: %s %s\n%s", err, soffice, strings.Join(args, " "), string(out))
	}

	converted := filepath.Join(tmpDir, base+".word.docx")
	if _, err := os.Stat(converted); err != nil {
		alt := filepath.Join(tmpDir, base+".docx")
		if _, altErr := os.Stat(alt); altErr == nil {
			converted = alt
		} else {
			return fmt.Errorf("converted DOCX not found. Expected: %s or %s\nLibreOffice output:\n%s", converted, alt, string(out))
		}
	}

	data, err := os.ReadFile(converted)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(docxPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(docxPath, data, 0644)
}

func findLibreOffice() (string, error) {
	for _, name := range []string{"libreoffice", "soffice"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("LibreOffice not found. Install it with: sudo apt-get install -y libreoffice")
}

func writeWordHTML(src string, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	html := string(data)

	html = removeToolbarForWord(html)
	html = removeDownloadWordScript(html)
	html = stripGoogleDocsUnsafeHTML(html)

	wordCSS := `
<style>
@page {
  size: A4;
  margin: 12mm 12mm 16mm 12mm;
}

html,
body {
  margin: 0;
  padding: 0;
  background: #ffffff;
  font-family: Ubuntu, Arial, sans-serif;
  font-size: 11pt;
  color: #111827;
}

.page {
  width: 186mm;
  min-height: 270mm;
  margin: 0 auto;
  page-break-after: always;
  position: relative;
  overflow: hidden;
  box-shadow: none !important;
  border: none !important;
  background: #ffffff;
}

.page-inner {
  padding: 12mm 12mm 22mm 12mm;
  box-sizing: border-box;
  position: relative;
  z-index: 2;
}

.footer {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
}

.watermark {
  opacity: 0.18 !important;
}

table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

th,
td {
  border: 1px solid #222;
  padding: 5px 6px;
  vertical-align: top;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.register-table th,
.register-table td {
  font-size: 8pt;
  line-height: 1.15;
}

.severity-table th,
.severity-table td {
  font-size: 9pt;
}

img {
  max-width: 100%;
}

..word-watermark-img {
  position: absolute;
  left: 24mm;
  top: 42mm;
  width: 150mm;
  opacity: 0.16;
  z-index: 0;
}

..word-footer-bg-img {
  position: absolute;
  left: 0;
  bottom: 0;
  width: 186mm;
  height: 22mm;
  z-index: 0;
  opacity: 1;
}

.footer {
  z-index: 5;
}
</style>
`

	html = strings.Replace(html, "</head>", wordCSS+"</head>", 1)

	return os.WriteFile(dst, []byte(html), 0644)
}

func removeToolbarForWord(html string) string {
	start := strings.Index(html, `<div class="toolbar`)
	if start == -1 {
		return html
	}

	end := strings.Index(html[start:], `</div>`)
	if end == -1 {
		return html
	}

	end = start + end + len(`</div>`)
	return html[:start] + html[end:]
}

func removeDownloadWordScript(html string) string {
	idx := strings.Index(html, "function downloadWordDoc()")
	if idx == -1 {
		return html
	}

	start := strings.LastIndex(html[:idx], "<script")
	end := strings.Index(html[idx:], "</script>")

	if start == -1 || end == -1 {
		return html
	}

	end = idx + end + len("</script>")

	return html[:start] + html[end:]
}

func injectWordBrandingImages(html string, reportDir string) string {
	watermark := findExistingAsset(reportDir, []string{
		"assets/watermark.png",
		"assets/watermark.jpg",
		"assets/watermark.jpeg",
	})

	footerBg := findExistingAsset(reportDir, []string{
		"assets/footer-bg.png",
		"assets/footer-bg.jpg",
		"assets/footer-bg.jpeg",
	})

	if watermark != "" && !strings.Contains(html, "word-watermark-img") {
		html = injectIntoReportSections(html, `<img class="word-watermark-img" src="`+watermark+`" alt="">`)
	}

	if footerBg != "" && !strings.Contains(html, "word-footer-bg-img") {
		html = injectIntoReportSections(html, `<img class="word-footer-bg-img" src="`+footerBg+`" alt="">`)
	}

	return html
}

func findExistingAsset(reportDir string, candidates []string) string {
	for _, rel := range candidates {
		full := filepath.Join(reportDir, rel)
		if _, err := os.Stat(full); err == nil {
			return rel
		}
	}
	return ""
}

func injectIntoReportSections(html string, injection string) string {
	markers := []string{
		`<section class="cover`,
		`<section class="flow-block`,
		`<section class="page`,
		`<div class="page`,
	}

	for _, marker := range markers {
		pos := 0
		for {
			idx := strings.Index(html[pos:], marker)
			if idx == -1 {
				break
			}
			idx += pos

			close := strings.Index(html[idx:], ">")
			if close == -1 {
				break
			}

			insertAt := idx + close + 1
			html = html[:insertAt] + injection + html[insertAt:]
			pos = insertAt + len(injection)
		}
	}

	return html
}

func injectIntoEveryPage(html string, injection string) string {
	for _, marker := range []string{`<div class="page`, `<section class="page`} {
		pos := 0
		for {
			idx := strings.Index(html[pos:], marker)
			if idx == -1 {
				break
			}
			idx += pos

			close := strings.Index(html[idx:], ">")
			if close == -1 {
				break
			}
			insertAt := idx + close + 1
			html = html[:insertAt] + injection + html[insertAt:]
			pos = insertAt + len(injection)
		}
	}
	return html
}

func injectIntoEveryFooter(html string, injection string) string {
	for _, marker := range []string{`<div class="footer`, `<footer class="footer`} {
		pos := 0
		for {
			idx := strings.Index(html[pos:], marker)
			if idx == -1 {
				break
			}
			idx += pos

			close := strings.Index(html[idx:], ">")
			if close == -1 {
				break
			}
			insertAt := idx + close + 1
			html = html[:insertAt] + injection + html[insertAt:]
			pos = insertAt + len(injection)
		}
	}
	return html
}

func stripGoogleDocsUnsafeHTML(html string) string {
	// Google Docs may reject DOCX generated from HTML containing huge data:image
	// CSS backgrounds. Keep the document clean and Word-compatible.
	replacements := []struct {
		pattern string
		repl    string
	}{
		{`(?is)<div[^>]*class="[^"]*\bwatermark\b[^"]*"[^>]*>.*?</div>`, ``},
		{`(?is)<img[^>]*class="[^"]*\bword-watermark-img\b[^"]*"[^>]*>`, ``},
		{`(?is)<img[^>]*class="[^"]*\bword-footer-bg-img\b[^"]*"[^>]*>`, ``},
		{`(?is)background-image\s*:\s*url\((['"]?)data:image/[^)]*\)\s*;?`, ``},
		{`(?is)url\((['"]?)data:image/[^)]*\)`, `none`},
		{`(?is)<script\b[^>]*>.*?</script>`, ``},
		{`(?is)<svg\b[^>]*>.*?</svg>`, ``},
		{`(?is)<img[^>]+src=["'][^"']+\.svg[^"']*["'][^>]*>`, ``},
		{`(?is)background-image\s*:\s*url\((['"]?)[^)]*\.svg[^)]*\)\s*;?`, ``},
	}

	for _, item := range replacements {
		html = regexp.MustCompile(item.pattern).ReplaceAllString(html, item.repl)
	}

	return html
}
