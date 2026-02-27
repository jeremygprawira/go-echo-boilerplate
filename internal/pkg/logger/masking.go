// Package logger provides credential masking utilities for secure logging.
// This file contains functions to mask sensitive data before logging.
package logger

import (
	"reflect"
	"strings"
	"sync"
)

// SensitiveFields is a list of field names that should be masked in logs.
// These are checked case-insensitively.
var SensitiveFields = []string{
	// Authentication & Authorization
	"password",
	"passwd",
	"pwd",
	"secret",
	"token",
	"auth",
	"authorization",
	"bearer",
	"api_key",
	"apikey",
	"api-key",
	"access_token",
	"refresh_token",
	"id_token",
	"session",
	"session_id",
	"sessionid",
	"cookie",

	// Credentials
	"credentials",
	"credential",
	"private_key",
	"privatekey",
	"public_key",
	"publickey",
	"cert",
	"certificate",

	// Payment & PII
	"credit_card",
	"creditcard",
	"card_number",
	"cvv",
	"cvc",
	"ssn",
	"social_security",

	// AWS & Cloud
	"aws_secret_access_key",
	"aws_access_key_id",
	"aws_session_token",

	// Database
	"db_password",
	"database_password",
	"connection_string",
	"connectionstring",

	// Custom headers
	"x-api-key",
	"x-auth-token",
	"x-access-token",
	"x-session-id",
}

// sensitiveFieldsMap is a map version of SensitiveFields for O(1) lookups.
// This is built automatically from SensitiveFields on first use.
var sensitiveFieldsMap map[string]bool
var sensitiveFieldsMapInit sync.Once

// initSensitiveFieldsMap builds the map from the slice (called once).
func initSensitiveFieldsMap() {
	sensitiveFieldsMap = make(map[string]bool, len(SensitiveFields))
	for _, field := range SensitiveFields {
		sensitiveFieldsMap[strings.ToLower(field)] = true
	}
}

const (
	// MaskString is the string used to replace sensitive values
	MaskString = "***MASKED***"

	// PartialMaskPrefix shows first N characters before masking
	PartialMaskPrefix = 4

	// MaxMaskingDataSize limits the size of data that can be masked (1MB)
	// Data larger than this will be replaced with a placeholder to prevent
	// excessive memory allocation during recursive masking operations.
	MaxMaskingDataSize = 1024 * 1024 // 1MB

	// MaxMaskingDepth limits recursion depth to prevent stack overflow
	MaxMaskingDepth = 10
)

// isSensitiveField checks if a field name is sensitive (case-insensitive).
// Uses a map for O(1) lookup performance instead of O(n) slice iteration.
func isSensitiveField(fieldName string) bool {
	// Initialize map on first use (thread-safe)
	sensitiveFieldsMapInit.Do(initSensitiveFieldsMap)

	lowerField := strings.ToLower(fieldName)

	// Check for exact matches first (fastest path)
	if sensitiveFieldsMap[lowerField] {
		return true
	}

	// Check for substring matches (for composite field names like "user_password")
	for sensitive := range sensitiveFieldsMap {
		if strings.Contains(lowerField, sensitive) {
			return true
		}
	}

	return false
}

// MaskSensitiveData recursively masks sensitive fields in the provided data.
// Supports maps, structs, slices, and primitive types.
//
// Memory Safety: Data larger than MaxMaskingDataSize (1MB) will be replaced
// with a placeholder to prevent excessive memory allocation.
//
// Example usage:
//
//	data := map[string]any{
//	    "username": "john",
//	    "password": "secret123",  // Will be masked
//	    "nested": map[string]any{
//	        "api_key": "sk_live_123",  // Will be masked recursively
//	    },
//	}
//	masked := logger.MaskSensitiveData(data)
func MaskSensitiveData(data interface{}) interface{} {
	// Estimate size and skip masking if too large
	if estimatedSize := estimateSize(data); estimatedSize > MaxMaskingDataSize {
		return "[DATA_TOO_LARGE_TO_MASK]"
	}

	return maskRecursive(data, 0, MaxMaskingDepth)
}

// estimateSize provides a rough estimate of data size in bytes.
// This is used to prevent masking operations on very large data structures.
func estimateSize(data interface{}) int {
	if data == nil {
		return 0
	}

	switch v := data.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	case map[string]interface{}:
		// Rough estimate: 100 bytes per entry
		return len(v) * 100
	case map[string]string:
		return len(v) * 50
	case []interface{}:
		return len(v) * 50
	case []map[string]interface{}:
		return len(v) * 100
	default:
		// For structs and other types, use reflection size estimate
		val := reflect.ValueOf(data)
		for val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return 0
			}
			val = val.Elem()
		}

		switch val.Kind() {
		case reflect.Struct:
			// Rough estimate: 50 bytes per field
			return val.NumField() * 50
		case reflect.Map:
			return val.Len() * 100
		case reflect.Slice, reflect.Array:
			return val.Len() * 50
		default:
			return 8 // Size of a pointer/primitive
		}
	}
}

// maskRecursive is the internal recursive masking function.
func maskRecursive(data interface{}, depth, maxDepth int) interface{} {
	// Prevent infinite recursion
	if depth > maxDepth {
		return data
	}

	if data == nil {
		return nil
	}

	// Handle different types
	switch v := data.(type) {
	case map[string]interface{}:
		return maskMap(v, depth, maxDepth)
	case map[string]string:
		return maskStringMap(v)
	case []interface{}:
		return maskSlice(v, depth, maxDepth)
	case []map[string]interface{}:
		return maskMapSlice(v, depth, maxDepth)
	default:
		// For structs and other types, use reflection
		return maskWithReflection(data, depth, maxDepth)
	}
}

// maskMap masks sensitive fields in a map[string]interface{}.
func maskMap(m map[string]interface{}, depth, maxDepth int) map[string]interface{} {
	masked := make(map[string]interface{}, len(m))

	for key, value := range m {
		if isSensitiveField(key) {
			masked[key] = maskValue(value)
		} else {
			masked[key] = maskRecursive(value, depth+1, maxDepth)
		}
	}

	return masked
}

// maskStringMap masks sensitive fields in a map[string]string.
func maskStringMap(m map[string]string) map[string]string {
	masked := make(map[string]string, len(m))

	for key, value := range m {
		if isSensitiveField(key) {
			masked[key] = MaskString
		} else {
			masked[key] = value
		}
	}

	return masked
}

// maskSlice masks sensitive data in a slice.
func maskSlice(s []interface{}, depth, maxDepth int) []interface{} {
	masked := make([]interface{}, len(s))

	for i, item := range s {
		masked[i] = maskRecursive(item, depth+1, maxDepth)
	}

	return masked
}

// maskMapSlice masks sensitive data in a slice of maps.
func maskMapSlice(s []map[string]interface{}, depth, maxDepth int) []map[string]interface{} {
	masked := make([]map[string]interface{}, len(s))

	for i, item := range s {
		masked[i] = maskMap(item, depth+1, maxDepth)
	}

	return masked
}

// maskWithReflection uses reflection to mask struct fields.
func maskWithReflection(data interface{}, depth, maxDepth int) interface{} {
	val := reflect.ValueOf(data)

	// Dereference pointers
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		return maskStruct(val, depth, maxDepth)
	case reflect.Map:
		return maskReflectMap(val, depth, maxDepth)
	case reflect.Slice, reflect.Array:
		return maskReflectSlice(val, depth, maxDepth)
	default:
		return data
	}
}

// maskStruct masks sensitive fields in a struct using reflection.
func maskStruct(val reflect.Value, depth, maxDepth int) map[string]interface{} {
	result := make(map[string]interface{})
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name (prefer json tag)
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Check if sensitive
		if isSensitiveField(fieldName) {
			result[fieldName] = MaskString
		} else {
			result[fieldName] = maskRecursive(fieldValue.Interface(), depth+1, maxDepth)
		}
	}

	return result
}

// maskReflectMap masks a map using reflection.
func maskReflectMap(val reflect.Value, depth, maxDepth int) interface{} {
	result := make(map[string]interface{})

	iter := val.MapRange()
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		keyStr := ""
		if key.Kind() == reflect.String {
			keyStr = key.String()
		} else {
			keyStr = key.String() // Fallback
		}

		if isSensitiveField(keyStr) {
			result[keyStr] = MaskString
		} else {
			result[keyStr] = maskRecursive(value.Interface(), depth+1, maxDepth)
		}
	}

	return result
}

// maskReflectSlice masks a slice using reflection.
func maskReflectSlice(val reflect.Value, depth, maxDepth int) interface{} {
	result := make([]interface{}, val.Len())

	for i := 0; i < val.Len(); i++ {
		result[i] = maskRecursive(val.Index(i).Interface(), depth+1, maxDepth)
	}

	return result
}

// maskValue masks a single value (for sensitive fields).
func maskValue(value interface{}) interface{} {
	// For strings, optionally show partial value
	if str, ok := value.(string); ok {
		if len(str) > PartialMaskPrefix {
			return str[:PartialMaskPrefix] + "..." + MaskString
		}
		return MaskString
	}

	// For other types, just return mask string
	return MaskString
}

// AddSensitiveField adds a custom field name to the sensitive fields list.
// This allows you to add application-specific sensitive fields.
//
// Thread-safe: This function rebuilds the internal map after adding the field.
//
// Example:
//
//	logger.AddSensitiveField("internal_token")
//	logger.AddSensitiveField("company_secret")
func AddSensitiveField(fieldName string) {
	SensitiveFields = append(SensitiveFields, strings.ToLower(fieldName))
	// Rebuild the map to include the new field
	initSensitiveFieldsMap()
}

// AddSensitiveFields adds multiple custom field names at once.
func AddSensitiveFields(fieldNames ...string) {
	for _, name := range fieldNames {
		AddSensitiveField(name)
	}
}

// MaskHeaders masks sensitive HTTP headers.
// Commonly used for logging request/response headers.
//
// Example:
//
//	headers := map[string]string{
//	    "Content-Type": "application/json",
//	    "Authorization": "Bearer token123",  // Will be masked
//	}
//	masked := logger.MaskHeaders(headers)
func MaskHeaders(headers map[string]string) map[string]string {
	return maskStringMap(headers)
}

// MaskHeadersInterface masks sensitive HTTP headers (interface{} version).
func MaskHeadersInterface(headers map[string]interface{}) map[string]interface{} {
	return maskMap(headers, 0, 10)
}
