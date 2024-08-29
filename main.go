package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/KyleBanks/dockerstats"
	"github.com/xuri/excelize/v2"
)

// convertToMB converts a string with units (e.g., "2GiB", "500MB") to MB.
func convertToMB(valueStr string) (float64, error) {
	var value float64
	var unit string

	// Trim any spaces
	valueStr = strings.TrimSpace(valueStr)

	// Check for unit and extract numeric value
	if strings.Contains(valueStr, "GiB") {
		valueStr = strings.TrimSuffix(valueStr, "GiB")
		unit = "GiB"
	} else if strings.Contains(valueStr, "GB") {
		valueStr = strings.TrimSuffix(valueStr, "GB")
		unit = "GB"
	} else if strings.Contains(valueStr, "MiB") {
		valueStr = strings.TrimSuffix(valueStr, "MiB")
		unit = "MiB"
	} else if strings.Contains(valueStr, "MB") {
		valueStr = strings.TrimSuffix(valueStr, "MB")
		unit = "MB"
	} else if strings.Contains(valueStr, "kB") {
		valueStr = strings.TrimSuffix(valueStr, "kB")
		unit = "KB"
	} else if strings.Contains(valueStr, "B") {
		valueStr = strings.TrimSuffix(valueStr, "B")
		unit = "B"
	} else {
		unit = "unknown"
	}

	// Parse numeric value
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}

	// Convert to MB
	switch unit {
	case "GiB", "GB":
		value *= 1024 // Convert GiB or GB to MB
	case "kB", "KB":
		value /= 1024 // Convert kB or KB to MB
	case "B":
		value /= (1024 * 1024) // Convert B to MB
	}

	return value, nil
}

func main() {
	// Parse command line arguments for container ID and name
	containerID := flag.String("container-id", "", "ID of the Docker container to monitor")
	containerName := flag.String("container-name", "", "Name of the Docker container to monitor")
	flag.Parse()

	if *containerID == "" {
		fmt.Println("Please provide a container ID using the -container-id flag.")
		return
	}

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTSTP) // Listen for Ctrl + Z

	// Generate filename with local date time and container name
	now := time.Now().Format("20060102-150405")
	fileName := fmt.Sprintf("%s-%s-stats.xlsx", now, *containerName)

	// Create a new Excel file
	f := excelize.NewFile()
	// defer func() {
	// 	if err := f.SaveAs(fileName); err != nil {
	// 		fmt.Println("Error saving Excel file:", err)
	// 	} else {
	// 		fmt.Println("Excel file saved successfully:", fileName)
	// 	}
	// }()

	// Set headers
	headers := []string{"Timestamp", "CPU Usage (%)", "Memory (MB)", "Block IO Read (MB)", "Block IO Write (MB)"}
	if err := f.SetSheetRow("Sheet1", "A1", &headers); err != nil {
		fmt.Println("Error setting headers:", err)
		return
	}

	// Monitor Docker stats
	m := dockerstats.NewMonitor()
	row := 2

	go func() {
		for res := range m.Stream {
			if res.Error != nil {
				fmt.Println("Error streaming stats:", res.Error)
				return
			}

			for _, s := range res.Stats {
				if s.Container != *containerID {
					continue
				}

				cpuUsageStr := strings.TrimSuffix(s.CPU, "%")
				cpuUsageVal, err := strconv.ParseFloat(cpuUsageStr, 64)
				if err != nil {
					fmt.Println("Error converting CpuUsage value:", err)
				}
				memoryStrVal := strings.Split(s.Memory.Raw, "/")[0]
				memoryVal, err := convertToMB(memoryStrVal)
				if err != nil {
					fmt.Println("Error converting memory value:", err)
					continue
				}
				blockIo := strings.Split(s.IO.Block, "/")
				blockIoReadVal, err := convertToMB(blockIo[0])
				if err != nil {
					fmt.Println("Error converting block IO read value:", err)
					continue
				}
				blockIoWriteVal, err := convertToMB(blockIo[1])
				if err != nil {
					fmt.Println("Error converting block IO write value:", err)
					continue
				}

				// Write to Excel
				data := []interface{}{
					time.Now().Format(time.RFC3339),
					cpuUsageVal,
					memoryVal,
					blockIoReadVal,
					blockIoWriteVal,
				}
				log.Println(data)
				if err := f.SetSheetRow("Sheet1", fmt.Sprintf("A%d", row), &data); err != nil {
					fmt.Println("Error writing data to Excel file:", err)
					continue
				}
				row++
			}
		}
	}()

	// Wait for Ctrl + Z (SIGTSTP)
	<-sigCh

	fmt.Println("Ctrl + Z detected. Saving and exiting...")

	// Save the file before exiting
	if err := f.SaveAs(fileName); err != nil {
		fmt.Println("Error saving Excel file:", err)
	} else {
		fmt.Println("Excel file saved successfully:", fileName)
	}
}
