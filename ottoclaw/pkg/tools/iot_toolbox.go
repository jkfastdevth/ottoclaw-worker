package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type IoTToolbox struct{}

func NewIoTToolbox() *IoTToolbox {
	return &IoTToolbox{}
}

func (t *IoTToolbox) Name() string {
	return "siam_iot"
}

func (t *IoTToolbox) Description() string {
	return "Specialized high-level IoT tools for sensor interaction and hardware control. Actions: read_sensor (reads data from supported sensors), toggle_gpio (control GPIO pins). Linux only."
}

func (t *IoTToolbox) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{"read_sensor", "toggle_gpio"},
				"description": "The IoT action to perform.",
			},
			"sensor_type": map[string]any{
				"type": "string",
				"enum": []string{"aht20", "bmp280", "battery", "thermal"},
				"description": "Type of sensor to read. Required for 'read_sensor'.",
			},
			"pin": map[string]any{
				"type": "integer",
				"description": "GPIO pin number (BCM). Required for 'toggle_gpio'.",
			},
			"value": map[string]any{
				"type": "integer",
				"enum": []int{0, 1},
				"description": "Value to set (0 for LOW, 1 for HIGH). Required for 'toggle_gpio'.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *IoTToolbox) Execute(ctx context.Context, args map[string]any) *ToolResult {
	if runtime.GOOS != "linux" {
		return ErrorResult("IoT tools are only supported on Linux/Android.")
	}

	action, _ := args["action"].(string)

	switch action {
	case "read_sensor":
		sensorType, _ := args["sensor_type"].(string)
		switch sensorType {
		case "aht20":
			return UserResult("AHT20 Sensor Data: Temp=26.5°C, Humidity=45.2% (Simulated)")
		case "battery":
			out, err := exec.Command("cat", "/sys/class/power_supply/battery/capacity").Output()
			if err != nil {
				// Try alternative BAT0 path
				out, err = exec.Command("cat", "/sys/class/power_supply/BAT0/capacity").Output()
			}
			if err != nil {
				return ErrorResult("Could not read battery: " + err.Error())
			}
			return UserResult(fmt.Sprintf("Battery Capacity: %s%%", strings.TrimSpace(string(out))))
		case "thermal":
			out, err := exec.Command("cat", "/sys/class/thermal/thermal_zone0/temp").Output()
			if err != nil {
				return ErrorResult("Could not read thermal data: " + err.Error())
			}
			return UserResult(fmt.Sprintf("SoC Temperature: %s (millidegrees)", strings.TrimSpace(string(out))))
		default:
			return ErrorResult("Unsupported sensor type: " + sensorType)
		}

	case "toggle_gpio":
		pin, okPin := args["pin"].(float64)
		value, okVal := args["value"].(float64)
		if !okPin || !okVal {
			return ErrorResult("pin and value are required for toggle_gpio")
		}
		
		// Attempting to use sysfs. Requires prior 'export' of the pin if not using helper libraries.
		cmd := exec.Command("sh", "-c", fmt.Sprintf("echo %d > /sys/class/gpio/gpio%d/value", int(value), int(pin)))
		err := cmd.Run()
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to toggle GPIO %d: %v (Note: may require root or explicit pin export)", int(pin), err))
		}
		return UserResult(fmt.Sprintf("GPIO %d set to %d", int(pin), int(value)))

	default:
		return ErrorResult("Unknown action: " + action)
	}
}
