package main

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestWorkbookImportsActiveSheetByRelationship(t *testing.T) {
	files := map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView activeTab="1"/></bookViews><sheets><sheet name="Ignored" r:id="rId1"/><sheet name="Rules" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="worksheets/sheet9.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>ignored</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet9.xml":   `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>active</t></is></c></row></sheetData></worksheet>`,
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, value := range files {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(value)); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	rows, e := parseXLSX(buf.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != "active" {
		t.Fatalf("wrong sheet: %v", rows)
	}
}
