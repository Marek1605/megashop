package importer

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

// FeedParser - Parser pre XML, CSV a JSON feedy
type FeedParser struct {
	URL          string
	Type         string
	XMLItemPath  string
	CSVDelimiter string
	CSVHasHeader bool
	Timeout      time.Duration
	MaxBytes     int64
	UserAgent    string
}

// ParseResult - Výsledok parsovania
type ParseResult struct {
	Items       []map[string]interface{} `json:"items"`
	Fields      []string                 `json:"fields"`
	TotalCount  int                      `json:"total_count"`
	ItemPath    string                   `json:"item_path"`
	Encoding    string                   `json:"encoding"`
	FeedType    string                   `json:"feed_type"`
	Error       string                   `json:"error,omitempty"`
	ParsedBytes int64                    `json:"parsed_bytes"`
}

// AutoMapping - Automatické mapovanie
type AutoMapping struct {
	SourceField string  `json:"source_field"`
	TargetField string  `json:"target_field"`
	Confidence  float64 `json:"confidence"`
}

// NewFeedParser vytvorí nový parser
func NewFeedParser(url, feedType string) *FeedParser {
	return &FeedParser{
		URL:          url,
		Type:         feedType,
		XMLItemPath:  "SHOPITEM",
		CSVDelimiter: ";",
		CSVHasHeader: true,
		Timeout:      5 * time.Minute,
		MaxBytes:     500 * 1024 * 1024,
		UserAgent:    "EshopBuilder/3.0",
	}
}

// Download stiahne feed
func (p *FeedParser) Download() ([]byte, error) {
	client := &http.Client{Timeout: p.Timeout}

	req, err := http.NewRequest("GET", p.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	req.Header.Set("User-Agent", p.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body

	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip error: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	data, err := io.ReadAll(io.LimitReader(reader, p.MaxBytes))
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	return data, nil
}

// DownloadPartial stiahne len časť feedu pre preview
func (p *FeedParser) DownloadPartial(maxBytes int64) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", p.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", p.UserAgent)
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", maxBytes-1))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	return data, nil
}

// Preview - Náhľad na feed
func (p *FeedParser) Preview(limit int) (*ParseResult, error) {
	data, err := p.DownloadPartial(100 * 1024)
	if err != nil {
		return nil, err
	}

	// Auto-detect type
	if p.Type == "" {
		p.Type = p.detectType(data)
	}

	switch p.Type {
	case "xml":
		return p.previewXML(data, limit)
	case "csv":
		return p.previewCSV(data, limit)
	case "json":
		return p.previewJSON(data, limit)
	default:
		return nil, fmt.Errorf("unsupported feed type: %s", p.Type)
	}
}

func (p *FeedParser) detectType(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "xml"
	}

	switch trimmed[0] {
	case '<':
		return "xml"
	case '[', '{':
		return "json"
	default:
		return "csv"
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// XML PARSING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (p *FeedParser) previewXML(data []byte, limit int) (*ParseResult, error) {
	result := &ParseResult{
		Items:       []map[string]interface{}{},
		Fields:      []string{},
		FeedType:    "xml",
		ParsedBytes: int64(len(data)),
	}

	// Sanitize and fix partial XML
	data = p.sanitizeXML(data)
	data = p.fixPartialXML(data)

	// Detect encoding
	result.Encoding = p.detectEncoding(data)

	// Detect item path if not set
	if p.XMLItemPath == "" {
		p.XMLItemPath = p.detectXMLItemPath(data)
	}
	result.ItemPath = p.XMLItemPath

	// Parse
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel

	fieldsMap := make(map[string]bool)
	count := 0
	currentItem := make(map[string]interface{})
	inItem := false
	itemDepth := 0
	currentPath := []string{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		switch t := token.(type) {
		case xml.StartElement:
			name := t.Name.Local
			currentPath = append(currentPath, name)

			if strings.EqualFold(name, p.XMLItemPath) {
				inItem = true
				itemDepth = len(currentPath)
				currentItem = make(map[string]interface{})
			}

		case xml.EndElement:
			name := t.Name.Local

			if inItem && strings.EqualFold(name, p.XMLItemPath) && len(currentPath) == itemDepth {
				if len(currentItem) > 0 && count < limit {
					result.Items = append(result.Items, currentItem)
					for k := range currentItem {
						fieldsMap[k] = true
					}
				}
				count++
				inItem = false
				currentItem = make(map[string]interface{})
			}

			if len(currentPath) > 0 {
				currentPath = currentPath[:len(currentPath)-1]
			}

		case xml.CharData:
			if inItem && len(currentPath) > itemDepth {
				fieldName := currentPath[len(currentPath)-1]
				value := strings.TrimSpace(string(t))
				if value != "" {
					currentItem[fieldName] = value
				}
			}
		}

		if count >= limit*2 {
			break
		}
	}

	result.TotalCount = count

	for f := range fieldsMap {
		result.Fields = append(result.Fields, f)
	}

	return result, nil
}

func (p *FeedParser) ParseXMLFull(data []byte, callback func(item map[string]interface{}) error) error {
	data = p.sanitizeXML(data)

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel

	currentItem := make(map[string]interface{})
	inItem := false
	itemDepth := 0
	currentPath := []string{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		switch t := token.(type) {
		case xml.StartElement:
			name := t.Name.Local
			currentPath = append(currentPath, name)

			if strings.EqualFold(name, p.XMLItemPath) {
				inItem = true
				itemDepth = len(currentPath)
				currentItem = make(map[string]interface{})
			}

		case xml.EndElement:
			name := t.Name.Local

			if inItem && strings.EqualFold(name, p.XMLItemPath) && len(currentPath) == itemDepth {
				if err := callback(currentItem); err != nil {
					return err
				}
				inItem = false
				currentItem = make(map[string]interface{})
			}

			if len(currentPath) > 0 {
				currentPath = currentPath[:len(currentPath)-1]
			}

		case xml.CharData:
			if inItem && len(currentPath) > itemDepth {
				fieldName := currentPath[len(currentPath)-1]
				value := strings.TrimSpace(string(t))
				if value != "" {
					currentItem[fieldName] = value
				}
			}
		}
	}

	return nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CSV PARSING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (p *FeedParser) previewCSV(data []byte, limit int) (*ParseResult, error) {
	result := &ParseResult{
		Items:       []map[string]interface{}{},
		Fields:      []string{},
		FeedType:    "csv",
		ParsedBytes: int64(len(data)),
	}

	// Detect delimiter
	if p.CSVDelimiter == "" {
		p.CSVDelimiter = p.detectCSVDelimiter(data)
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = rune(p.CSVDelimiter[0])
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return result, nil
	}

	// First row as headers
	headers := records[0]
	result.Fields = headers

	for i, record := range records[1:] {
		if i >= limit {
			break
		}
		item := make(map[string]interface{})
		for j, value := range record {
			if j < len(headers) {
				item[headers[j]] = value
			}
		}
		result.Items = append(result.Items, item)
	}

	result.TotalCount = len(records) - 1
	return result, nil
}

func (p *FeedParser) ParseCSVFull(data []byte, callback func(item map[string]interface{}) error) error {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = rune(p.CSVDelimiter[0])
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	if len(records) == 0 {
		return nil
	}

	headers := records[0]

	for _, record := range records[1:] {
		item := make(map[string]interface{})
		for j, value := range record {
			if j < len(headers) {
				item[headers[j]] = value
			}
		}
		if err := callback(item); err != nil {
			return err
		}
	}

	return nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// JSON PARSING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (p *FeedParser) previewJSON(data []byte, limit int) (*ParseResult, error) {
	result := &ParseResult{
		Items:       []map[string]interface{}{},
		Fields:      []string{},
		FeedType:    "json",
		ParsedBytes: int64(len(data)),
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	items := p.findJSONProductsArray(raw)
	if items == nil {
		return nil, fmt.Errorf("no products array found")
	}

	fieldsMap := make(map[string]bool)

	for i, item := range items {
		if i >= limit {
			break
		}

		if m, ok := item.(map[string]interface{}); ok {
			result.Items = append(result.Items, m)
			for k := range m {
				fieldsMap[k] = true
			}
		}
	}

	result.TotalCount = len(items)

	for f := range fieldsMap {
		result.Fields = append(result.Fields, f)
	}

	return result, nil
}

func (p *FeedParser) ParseJSONFull(data []byte, callback func(item map[string]interface{}) error) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	items := p.findJSONProductsArray(raw)
	if items == nil {
		return fmt.Errorf("no products array found")
	}

	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			if err := callback(m); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *FeedParser) findJSONProductsArray(data interface{}) []interface{} {
	if arr, ok := data.([]interface{}); ok {
		return arr
	}

	if obj, ok := data.(map[string]interface{}); ok {
		keys := []string{"products", "items", "offers", "data", "results", "SHOPITEM"}
		for _, key := range keys {
			if arr, ok := obj[key].([]interface{}); ok {
				return arr
			}
			if arr, ok := obj[strings.ToLower(key)].([]interface{}); ok {
				return arr
			}
		}
	}

	return nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (p *FeedParser) sanitizeXML(data []byte) []byte {
	// Remove BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	// Remove invalid chars
	result := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`).ReplaceAll(data, []byte{})

	return result
}

func (p *FeedParser) fixPartialXML(data []byte) []byte {
	str := string(data)

	closeTagLower := "</" + strings.ToLower(p.XMLItemPath) + ">"
	lastClose := strings.LastIndex(strings.ToLower(str), closeTagLower)

	if lastClose > 0 {
		str = str[:lastClose+len(closeTagLower)]
	}

	// Close root tags
	if !strings.HasSuffix(strings.TrimSpace(str), ">") {
		str += ">"
	}

	return []byte(str)
}

func (p *FeedParser) detectEncoding(data []byte) string {
	if matches := regexp.MustCompile(`encoding=["']([^"']+)["']`).FindSubmatch(data); len(matches) > 1 {
		return string(matches[1])
	}
	return "UTF-8"
}

func (p *FeedParser) detectXMLItemPath(data []byte) string {
	paths := []string{"SHOPITEM", "product", "item", "offer", "entry", "PRODUCT", "ITEM"}
	lowerData := strings.ToLower(string(data))

	for _, path := range paths {
		if strings.Contains(lowerData, "<"+strings.ToLower(path)) {
			return path
		}
	}

	return "SHOPITEM"
}

func (p *FeedParser) detectCSVDelimiter(data []byte) string {
	delimiters := []string{";", ",", "\t", "|"}
	lines := strings.Split(string(data), "\n")

	if len(lines) < 2 {
		return ";"
	}

	maxScore := 0
	best := ";"

	for _, d := range delimiters {
		firstCount := strings.Count(lines[0], d)
		if firstCount > 0 {
			score := 0
			for _, line := range lines[1:5] {
				if strings.Count(line, d) == firstCount {
					score++
				}
			}
			if score > maxScore {
				maxScore = score
				best = d
			}
		}
	}

	return best
}

// AutoDetectMappings - Automatické mapovanie polí
func (p *FeedParser) AutoDetectMappings(fields []string) []AutoMapping {
	mappings := []AutoMapping{}

	patterns := map[string][][]string{
		"ean":           {{"^ean$", "^ean13$", "^gtin$", "^barcode$", "^ITEM_ID$"}},
		"sku":           {{"^sku$", "^productno$", "^kod$", "^item_id$", "^ITEMGROUP_ID$"}},
		"external_id":   {{"^id$", "^external_id$", "^ext_id$"}},
		"title":         {{"^productname$", "^product$", "^title$", "^name$", "^nazov$", "^PRODUCTNAME$"}},
		"description":   {{"^description$", "^popis$", "^desc$", "^DESCRIPTION$"}},
		"price":         {{"^price_vat$", "^price$", "^cena$", "^PRICE_VAT$"}},
		"image_url":     {{"^imgurl$", "^img_url$", "^image$", "^foto$", "^IMGURL$"}},
		"gallery_images":{{"^imgurl_alternative$", "^gallery$", "^images$"}},
		"affiliate_url": {{"^url$", "^link$", "^product_url$", "^URL$"}},
		"category":      {{"^categorytext$", "^category$", "^kategoria$", "^CATEGORYTEXT$"}},
		"brand":         {{"^manufacturer$", "^brand$", "^vyrobca$", "^MANUFACTURER$"}},
		"stock_quantity":{{"^stock$", "^quantity$", "^sklad$", "^STOCK_QUANTITY$"}},
		"delivery_time": {{"^delivery$", "^delivery_date$", "^dodanie$", "^DELIVERY_DATE$"}},
		"attributes":    {{"^param$", "^params$", "^PARAM$"}},
	}

	for _, field := range fields {
		fieldLower := strings.ToLower(field)

		for target, patternGroups := range patterns {
			for _, group := range patternGroups {
				for _, pattern := range group {
					if matched, _ := regexp.MatchString("(?i)"+pattern, fieldLower); matched {
						mappings = append(mappings, AutoMapping{
							SourceField: field,
							TargetField: target,
							Confidence:  0.9,
						})
						goto nextField
					}
				}
			}
		}
	nextField:
	}

	return mappings
}
