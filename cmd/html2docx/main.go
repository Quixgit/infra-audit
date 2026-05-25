package main

import (
	"flag"
	"fmt"
	"os"

	"infra-audit/internal/report"
)

func main() {
	htmlPath := flag.String("html", "", "input HTML report path")
	docxPath := flag.String("docx", "", "output DOCX path")
	flag.Parse()

	if *htmlPath == "" || *docxPath == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/html2docx --html report.html --docx report.docx")
		os.Exit(2)
	}

	if err := report.ConvertHTMLToDOCX(*htmlPath, *docxPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Converted:", *docxPath)
}
