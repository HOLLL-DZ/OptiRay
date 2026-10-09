package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

func xmlText(s string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(s))
	return out.String()
}
func columnName(index int) string {
	s := ""
	for index++; index > 0; index = (index - 1) / 26 {
		s = string(rune('A'+(index-1)%26)) + s
	}
	return s
}
func xlsxBytes(rows [][]string) ([]byte, error) {
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for i, row := range rows {
		fmt.Fprintf(&sheet, `<row r="%d">`, i+1)
		for j, value := range row {
			fmt.Fprintf(&sheet, `<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, columnName(j), i+1, xmlText(value))
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	files := map[string]string{"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`, "_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`, "xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Rules" sheetId="1" r:id="rId1"/></sheets></workbook>`, "xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`, "xl/worksheets/sheet1.xml": sheet.String()}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f, e := writer.Create(name)
		if e != nil {
			return nil, e
		}
		if _, e = f.Write([]byte(files[name])); e != nil {
			return nil, e
		}
	}
	if e := writer.Close(); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}

type xlsxRich struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (r xlsxRich) value() string {
	s := r.Text
	for _, run := range r.Runs {
		s += run.Text
	}
	return s
}
func parseXLSX(blob []byte) ([][]string, error) {
	reader, e := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if e != nil {
		return nil, e
	}
	if len(reader.File) > 256 {
		return nil, fmt.Errorf("too many workbook files")
	}
	files := map[string]*zip.File{}
	sheets := []string{}
	var total uint64
	for _, f := range reader.File {
		total += f.UncompressedSize64
		if total > 40<<20 {
			return nil, fmt.Errorf("workbook is too large")
		}
		files[f.Name] = f
		if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
			sheets = append(sheets, f.Name)
		}
	}
	sort.Strings(sheets)
	if len(sheets) == 0 {
		return nil, fmt.Errorf("worksheet missing")
	}
	read := func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, fmt.Errorf("workbook file missing")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		b, e := io.ReadAll(io.LimitReader(r, (16<<20)+1))
		if len(b) > 16<<20 {
			return nil, fmt.Errorf("worksheet too large")
		}
		return b, e
	}
	selectedSheet := sheets[0]
	if files["xl/workbook.xml"] != nil && files["xl/_rels/workbook.xml.rels"] != nil {
		bookBytes, e := read("xl/workbook.xml")
		if e != nil {
			return nil, e
		}
		var book struct {
			Sheets []struct {
				ID string `xml:"id,attr"`
			} `xml:"sheets>sheet"`
			Views []struct {
				Active int `xml:"activeTab,attr"`
			} `xml:"bookViews>workbookView"`
		}
		if e = xml.Unmarshal(bookBytes, &book); e != nil {
			return nil, e
		}
		relBytes, e := read("xl/_rels/workbook.xml.rels")
		if e != nil {
			return nil, e
		}
		var relations struct {
			Items []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
				Mode   string `xml:"TargetMode,attr"`
			} `xml:"Relationship"`
		}
		if e = xml.Unmarshal(relBytes, &relations); e != nil {
			return nil, e
		}
		active := 0
		if len(book.Views) > 0 {
			active = book.Views[0].Active
		}
		if active < 0 || active >= len(book.Sheets) {
			active = 0
		}
		if len(book.Sheets) > 0 {
			found := false
			for _, rel := range relations.Items {
				if rel.ID != book.Sheets[active].ID {
					continue
				}
				if rel.Mode == "External" {
					return nil, fmt.Errorf("external worksheet unsupported")
				}
				target := path.Clean(path.Join("xl", rel.Target))
				if strings.HasPrefix(rel.Target, "/") {
					target = strings.TrimPrefix(path.Clean(rel.Target), "/")
				}
				if !strings.HasPrefix(target, "xl/worksheets/") || files[target] == nil {
					return nil, fmt.Errorf("invalid worksheet target")
				}
				selectedSheet = target
				found = true
				break
			}
			if !found {
				return nil, fmt.Errorf("worksheet relationship missing")
			}
		}
	}
	shared := []string{}
	if files["xl/sharedStrings.xml"] != nil {
		b, e := read("xl/sharedStrings.xml")
		if e != nil {
			return nil, e
		}
		var doc struct {
			Items []xlsxRich `xml:"si"`
		}
		if e = xml.Unmarshal(b, &doc); e != nil {
			return nil, e
		}
		for _, v := range doc.Items {
			shared = append(shared, v.value())
		}
	}
	b, e := read(selectedSheet)
	if e != nil {
		return nil, e
	}
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Reference string   `xml:"r,attr"`
				Type      string   `xml:"t,attr"`
				Value     string   `xml:"v"`
				Inline    xlsxRich `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if e = xml.Unmarshal(b, &sheet); e != nil {
		return nil, e
	}
	if len(sheet.Rows) > 100000 {
		return nil, fmt.Errorf("too many rule rows")
	}
	rows := [][]string{}
	for _, r := range sheet.Rows {
		row := []string{}
		for _, c := range r.Cells {
			index := 0
			for _, ch := range c.Reference {
				if ch < 'A' || ch > 'Z' {
					break
				}
				index = index*26 + int(ch-'A') + 1
			}
			if index == 0 {
				index = len(row) + 1
			}
			index--
			if index > 100 {
				return nil, fmt.Errorf("too many rule columns")
			}
			for len(row) <= index {
				row = append(row, "")
			}
			value := c.Value
			if c.Type == "inlineStr" {
				value = c.Inline.value()
			} else if c.Type == "s" {
				i, e := strconv.Atoi(value)
				if e != nil || i < 0 || i >= len(shared) {
					return nil, fmt.Errorf("invalid shared string index")
				}
				value = shared[i]
			}
			row[index] = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "false", "no", "off", "нет":
		return false
	default:
		return true
	}
}
func boolCell(m Object, key string) string {
	if v, exists := m[key]; exists && !boolean(v) {
		return "0"
	}
	return "1"
}
func routingRows(groups any, optirout bool) [][]string {
	rows := [][]string{{"Категория", "Группа", "Группа вкл (1/0)", "Правило", "Правило вкл (1/0)"}}
	if optirout {
		rows = [][]string{{"Группа", "Цвет", "Интерфейс", "Группа вкл (1/0)", "Тип правила", "Правило", "Правило вкл (1/0)"}}
		for _, v := range list(groups) {
			g := obj(v)
			rules := list(g["rules"])
			if len(rules) == 0 {
				rules = []any{Object{}}
			}
			for _, r := range rules {
				rule := obj(r)
				if text, ok := r.(string); ok {
					rule = Object{"rule": text}
				}
				rows = append(rows, []string{str(g["name"]), first(str(g["color"]), "#888888"), str(g["interface"]), boolCell(g, "enable"), first(str(rule["type"]), "namespace"), str(rule["rule"]), boolCell(rule, "enable")})
			}
		}
		return rows
	}
	for _, category := range []string{"block", "direct", "proxy"} {
		for _, v := range list(obj(groups)[category]) {
			g := obj(v)
			rules := list(g["rules"])
			if len(rules) == 0 {
				rules = []any{Object{}}
			}
			for _, v := range rules {
				r := obj(v)
				value := str(r["value"])
				if _, ok := v.(string); ok {
					value = str(v)
				}
				rows = append(rows, []string{category, str(g["title"]), boolCell(g, "enabled"), value, boolCell(r, "enabled")})
			}
		}
	}
	return rows
}
func groupsFromRows(rows [][]string, optirout bool) (any, error) {
	groups := []any{}
	categories := Object{"block": []any{}, "direct": []any{}, "proxy": []any{}}
	index := map[string]Object{}
	seen := map[string]bool{}
	for i, row := range rows {
		cell := func(n int, fallback string) string {
			if n >= len(row) {
				return fallback
			}
			return strings.TrimSpace(row[n])
		}
		name := cell(0, "")
		if i == 0 && (strings.ToLower(name) == "категория" || strings.ToLower(name) == "category" || strings.ToLower(name) == "группа" || strings.ToLower(name) == "group") {
			continue
		}
		if optirout {
			value := cell(5, "")
			if name == "" && value == "" {
				continue
			}
			color, iface := first(cell(1, ""), "#888888"), cell(2, "")
			key := name + "\x00" + color + "\x00" + iface
			g := index[key]
			if g == nil {
				id, e := optiroutID(seen, "")
				if e != nil {
					return nil, e
				}
				g = Object{"id": id, "name": first(name, "Импорт"), "color": color, "interface": iface, "enable": truthy(cell(3, "1")), "rules": []any{}}
				index[key] = g
				groups = append(groups, g)
			}
			if value != "" {
				id, e := optiroutID(seen, "")
				if e != nil {
					return nil, e
				}
				g["rules"] = append(list(g["rules"]), Object{"id": id, "name": "", "type": first(cell(4, ""), "namespace"), "rule": value, "enable": truthy(cell(6, "1"))})
			}
		} else {
			category := strings.ToLower(name)
			if category != "block" && category != "direct" && category != "proxy" {
				continue
			}
			title, value := cell(1, ""), cell(3, "")
			key := category + "\x00" + title
			g := index[key]
			if g == nil {
				id, e := optiroutID(seen, "")
				if e != nil {
					return nil, e
				}
				g = Object{"id": "g" + id, "title": title, "enabled": truthy(cell(2, "1")), "collapsed": false, "rules": []any{}}
				index[key] = g
				categories[category] = append(list(categories[category]), g)
			}
			if value != "" {
				g["rules"] = append(list(g["rules"]), Object{"value": value, "enabled": truthy(cell(4, "1"))})
			}
		}
	}
	for _, g := range index {
		if len(list(g["rules"])) == 0 {
			if optirout {
				id, e := optiroutID(seen, "")
				if e != nil {
					return nil, e
				}
				g["rules"] = []any{Object{"id": id, "name": "", "type": "namespace", "rule": "", "enable": true}}
			} else {
				g["rules"] = []any{Object{"value": "", "enabled": true}}
			}
		}
	}
	if optirout {
		return groups, nil
	}
	return categories, nil
}
