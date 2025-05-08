package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConvertData converts the given data to the specified format
func ConvertData(data interface{}, format string) (string, error) {
	switch format {
	case "json":
		return convertToJSON(data)
	case "yaml":
		return convertToYAML(data)
	case "csv":
		return convertToCSV(data)
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}
}

// convertToJSON converts data to JSON format
func convertToJSON(data interface{}) (string, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error converting to JSON: %v", err)
	}
	return string(jsonBytes), nil
}

// convertToYAML converts data to YAML format
func convertToYAML(data interface{}) (string, error) {
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error converting to YAML: %v", err)
	}
	return string(yamlBytes), nil
}

// convertToCSV converts data to CSV format
func convertToCSV(data interface{}) (string, error) {
	// Validate that data is a slice/array
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return "", fmt.Errorf("CSV conversion requires an array/slice of objects, got %s", v.Kind())
	}
	
	if v.Len() == 0 {
		return "", nil // Return empty string for empty array
	}
	
	// Get headers from the first element (assuming all elements have same structure)
	headers := []string{}
	headerMap := make(map[string]bool)
	
	// First pass: Gather all possible headers across all entries
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Interface()
		
		// Try to convert item to a map
		var itemMap map[string]interface{}
		
		switch typedItem := item.(type) {
		case map[string]interface{}:
			itemMap = typedItem
		case DataEntry:
			itemMap = typedItem
		default:
			return "", fmt.Errorf("CSV conversion requires array of maps/objects")
		}
		
		// Add all keys as headers
		for key := range itemMap {
			if !headerMap[key] {
				headerMap[key] = true
				headers = append(headers, key)
			}
		}
	}
	
	// Sort headers for consistent output
	sort.Strings(headers)
	
	// Build CSV
	var sb strings.Builder
	csvWriter := csv.NewWriter(&sb)
	
	// Write header row
	if err := csvWriter.Write(headers); err != nil {
		return "", fmt.Errorf("error writing CSV header: %v", err)
	}
	
	// Write data rows
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Interface()
		
		// Convert item to a map
		var itemMap map[string]interface{}
		
		switch typedItem := item.(type) {
		case map[string]interface{}:
			itemMap = typedItem
		case DataEntry:
			itemMap = typedItem
		default:
			return "", fmt.Errorf("unexpected type in array")
		}
		
		// Create row with values corresponding to headers
		row := make([]string, len(headers))
		for j, header := range headers {
			if val, ok := itemMap[header]; ok {
				row[j] = fmt.Sprintf("%v", val)
			} else {
				row[j] = "" // Empty string for missing values
			}
		}
		
		if err := csvWriter.Write(row); err != nil {
			return "", fmt.Errorf("error writing CSV row: %v", err)
		}
	}
	
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return "", fmt.Errorf("error flushing CSV writer: %v", err)
	}
	
	return sb.String(), nil
}