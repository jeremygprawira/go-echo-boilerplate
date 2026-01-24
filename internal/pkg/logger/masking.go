// Package logger provides credential masking utilities for secure logging.
// This file contains functions to mask sensitive data before logging.
package logger

import (
	"reflect"
	"strings"
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

const (
	// MaskString is the string used to replace sensitive values
	MaskString = "***MASKED***"

	// PartialMaskPrefix shows first N characters before masking
	PartialMaskPrefix = 4
)

// isSensitiveField checks if a field name is sensitive (case-insensitive).
func isSensitiveField(fieldName string) bool {
	lowerField := strings.ToLower(fieldName)

	for _, sensitive := range SensitiveFields {
		if strings.Contains(lowerField, strings.ToLower(sensitive)) {
			return true
		}
	}

	return false
}

// MaskSensitiveData recursively masks sensitive fields in the provided data.
// Supports maps, structs, slices, and primitive types.
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
	return maskRecursive(data, 0, 10) // Max depth of 10 to prevent infinite recursion
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
// Example:
//
//	logger.AddSensitiveField("internal_token")
//	logger.AddSensitiveField("company_secret")
func AddSensitiveField(fieldName string) {
	SensitiveFields = append(SensitiveFields, strings.ToLower(fieldName))
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
