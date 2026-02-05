package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed config.schema.json
var configSchemaJSON []byte

// schemaCompiler is a singleton for the compiled JSON schema.
var (
	compiledSchema    *jsonschema.Schema //nolint:gochecknoglobals // schema singleton
	schemaCompileOnce sync.Once          //nolint:gochecknoglobals // schema singleton
	errSchemaCompile  error              //nolint:gochecknoglobals // schema singleton
)

// ValidationError represents a single validation error with field and message.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult contains the result of configuration validation.
type ValidationResult struct { //nolint:errname // not applicable
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Error implements the error interface for ValidationResult.
func (vr ValidationResult) Error() string {
	if vr.Valid {
		return ""
	}

	var messages []string
	for _, err := range vr.Errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}

	return strings.Join(messages, "; ")
}

// Validator defines the interface for configuration validation.
// This interface allows for easy replacement with external validators in the future.
type Validator interface {
	Validate() ValidationResult
}

// getCompiledSchema returns the compiled JSON schema, compiling it on first use.
func getCompiledSchema() (*jsonschema.Schema, error) {
	schemaCompileOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.Draft = jsonschema.Draft7

		err := compiler.AddResource("config.schema.json", bytes.NewReader(configSchemaJSON))
		if err != nil {
			errSchemaCompile = fmt.Errorf("failed to add schema resource: %w", err)

			return
		}

		compiledSchema, errSchemaCompile = compiler.Compile("config.schema.json")
		if errSchemaCompile != nil {
			errSchemaCompile = fmt.Errorf("failed to compile schema: %w", errSchemaCompile)
		}
	})

	return compiledSchema, errSchemaCompile
}

// Validate performs comprehensive validation of the configuration.
// Returns ValidationResult with any validation errors found.
// This uses JSON schema validation for structure/type/range checks,
// plus custom validation for things that can't be expressed in JSON schema.
func (c *Config) Validate() ValidationResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// Convert config to JSON for schema validation
	jsonData, err := c.toJSON()
	if err != nil {
		addError(&result, "config", fmt.Sprintf("failed to serialize config: %v", err))
		result.Valid = false

		return result
	}

	// Validate against JSON schema
	schemaResult := validateJSONSchema(jsonData)
	if !schemaResult.Valid {
		result.Errors = append(result.Errors, schemaResult.Errors...)
	}

	// Custom validations that can't be expressed in JSON schema:
	// 1. Filesystem checks (baseDir exists)
	// 2. Cross-field comparisons (pulseMinFrequencyHz < pulseMaxFrequencyHz)
	// 3. Conditional requirements (Discord guildID required when token is set)

	if c.viper.App != nil {
		validateAppCustom(c.viper.App, &result)
	}

	if c.viper.Haptics != nil {
		validateHapticsCustom(c.viper.Haptics, &result)
	}

	if c.viper.PitRadio != nil && c.viper.PitRadio.Discord != nil {
		validateDiscordCustom(c.viper.PitRadio.Discord, &result)
	}

	result.Valid = len(result.Errors) == 0

	return result
}

// ValidateConfig validates JSON configuration data without modifying the current config.
// This method is useful for validating imported/uploaded configuration files.
func (c *Config) ValidateConfig(jsonData []byte) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// First, validate basic JSON syntax
	var rawData any

	err := json.Unmarshal(jsonData, &rawData)
	if err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "config",
				Message: fmt.Sprintf("failed to parse JSON: %v", err),
			}},
		}
	}

	// Validate against JSON schema
	schemaResult := validateJSONSchema(jsonData)
	if !schemaResult.Valid {
		result.Errors = append(result.Errors, schemaResult.Errors...)
	}

	// Unmarshal into our config structure for custom validation
	var tempConfig viperConfig

	err = json.Unmarshal(jsonData, &tempConfig)
	if err != nil {
		addError(&result, "config", fmt.Sprintf("failed to unmarshal config: %v", err))
		result.Valid = false

		return result
	}

	// Run custom validations
	if tempConfig.App != nil {
		validateAppCustom(tempConfig.App, &result)
	}

	if tempConfig.Haptics != nil {
		validateHapticsCustom(tempConfig.Haptics, &result)
	}

	if tempConfig.PitRadio != nil && tempConfig.PitRadio.Discord != nil {
		validateDiscordCustom(tempConfig.PitRadio.Discord, &result)
	}

	result.Valid = len(result.Errors) == 0

	return result
}

// validateJSONSchema validates JSON data against the embedded config schema.
func validateJSONSchema(jsonData []byte) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	schema, err := getCompiledSchema()
	if err != nil {
		addError(&result, "schema", fmt.Sprintf("failed to load schema: %v", err))
		result.Valid = false

		return result
	}

	var data any

	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		addError(&result, "config", fmt.Sprintf("failed to parse JSON: %v", err))
		result.Valid = false

		return result
	}

	err = schema.Validate(data)
	if err != nil {
		// Convert jsonschema validation errors to our format
		validationErr := &jsonschema.ValidationError{}

		ok := errors.As(err, &validationErr)
		if ok {
			extractValidationErrors(validationErr, &result)
		} else {
			addError(&result, "config", fmt.Sprintf("validation error: %v", err))
		}

		result.Valid = false
	}

	return result
}

// extractValidationErrors recursively extracts validation errors from jsonschema.ValidationError.
func extractValidationErrors(err *jsonschema.ValidationError, result *ValidationResult) {
	// If there are causes, recurse into them for more specific errors
	if len(err.Causes) > 0 {
		for _, cause := range err.Causes {
			extractValidationErrors(cause, result)
		}

		return
	}

	// Extract the field path from the instance location
	field := err.InstanceLocation
	if field == "" {
		field = "config"
	}

	// Clean up the field path (remove leading /)
	field = strings.TrimPrefix(field, "/")
	field = strings.ReplaceAll(field, "/", ".")

	addError(result, field, err.Message)
}

// toJSON converts the current config to JSON for schema validation.
func (c *Config) toJSON() ([]byte, error) {
	return json.Marshal(c.viper)
}

// addError is a helper to add validation errors.
func addError(result *ValidationResult, field, message string) {
	result.Errors = append(result.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// validateAppCustom performs custom validation for app settings that can't be done via schema.
func validateAppCustom(appCfg *app, result *ValidationResult) {
	// Validate base directory exists if specified
	if appCfg.BaseDir != "" {
		_, err := os.Stat(appCfg.BaseDir)
		if err != nil {
			addError(result, "app.baseDir", "directory does not exist or is not accessible: "+appCfg.BaseDir)
		}
	}
}

// validateHapticsCustom performs custom validation for haptics settings.
func validateHapticsCustom(hap *haptics, result *ValidationResult) {
	// Validate min < max for pulse frequency
	if hap.PulseMinFrequencyHz >= hap.PulseMaxFrequencyHz {
		addError(result, "haptics.pulseMinFrequencyHz", "pulseMinFrequencyHz must be less than pulseMaxFrequencyHz")
	}
}

// validateDiscordCustom performs custom validation for Discord settings.
func validateDiscordCustom(discordCfg *discord, result *ValidationResult) {
	// If Discord token is set, guildID is required
	if discordCfg.Token != "" && discordCfg.GuildID == "" {
		addError(result, "pitRadio.discord.guildID", "guildID is required when token is set")
	}
}
