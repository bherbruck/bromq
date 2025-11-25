package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github/bromq-dev/bromq/internal/config"

	"github.com/invopop/jsonschema"
)

func main() {
	// Create reflector with custom settings
	reflector := &jsonschema.Reflector{
		Anonymous:                  false, // Don't inline definitions
		DoNotReference:             false, // Use $ref for reusable types
		AllowAdditionalProperties:  false, // Strict validation
		RequiredFromJSONSchemaTags: true,  // Use jsonschema:"required" tags
	}

	// Generate schema from Config struct
	schema := reflector.Reflect(&config.Config{})

	// Add metadata
	schema.ID = "https://bromq.dev/schema/config/v1/schema.json"
	schema.Title = "BroMQ Configuration"
	schema.Description = "Configuration file for BroMQ MQTT broker with automatic provisioning support"

	// Add top-level example
	schema.Examples = []interface{}{
		map[string]interface{}{
			"users": []map[string]interface{}{
				{
					"username":    "sensor_user",
					"password":    "${SENSOR_PASSWORD}",
					"description": "IoT sensor devices",
				},
			},
			"acl_rules": []map[string]interface{}{
				{
					"username":   "sensor_user",
					"topic":      "sensors/${username}/#",
					"permission": "pubsub",
				},
			},
			"bridges": []map[string]interface{}{
				{
					"name":     "cloud-bridge",
					"host":     "${CLOUD_MQTT_HOST:-mqtt.example.com}",
					"port":     1883,
					"username": "${CLOUD_USER}",
					"password": "${CLOUD_PASSWORD}",
					"topics": []map[string]interface{}{
						{
							"local":     "sensors/#",
							"remote":    "edge/sensors/#",
							"direction": "out",
							"qos":       1,
						},
					},
				},
			},
			"scripts": []map[string]interface{}{
				{
					"name":    "logger",
					"enabled": true,
					"file":    "./scripts/logger.js",
					"triggers": []map[string]interface{}{
						{
							"type":  "on_publish",
							"topic": "#",
						},
					},
				},
			},
		},
	}

	// Output formatted JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding schema: %v\n", err)
		os.Exit(1)
	}
}
