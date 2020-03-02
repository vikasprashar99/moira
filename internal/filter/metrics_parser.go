package filter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	moira2 "github.com/moira-alert/moira/internal/moira"
)

// ParsedMetric represents a result of ParseMetric.
type ParsedMetric struct {
	Metric    string
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp int64
}

// ParseMetric parses metric from string
// supported format: "<metricString> <valueFloat64> <timestampInt64>"
func ParseMetric(input []byte) (*ParsedMetric, error) {
	if !isPrintableASCII(input) {
		return nil, fmt.Errorf("non-ascii or non-printable chars in metric name: '%s'", input)
	}

	var metricBytes, valueBytes, timestampBytes []byte
	inputScanner := moira2.NewBytesScanner(input, ' ')
	if !inputScanner.HasNext() {
		return nil, fmt.Errorf("too few space-separated items: '%s'", input)
	}
	metricBytes = inputScanner.Next()
	if !inputScanner.HasNext() {
		return nil, fmt.Errorf("too few space-separated items: '%s'", input)
	}
	valueBytes = inputScanner.Next()
	if !inputScanner.HasNext() {
		return nil, fmt.Errorf("too few space-separated items: '%s'", input)
	}
	timestampBytes = inputScanner.Next()
	if inputScanner.HasNext() {
		return nil, fmt.Errorf("too many space-separated items: '%s'", input)
	}

	name, labels, err := parseNameAndLabels(metricBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse metric: '%s' (%s)", input, err)
	}

	value, err := parseFloat(valueBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse value: '%s' (%s)", input, err)
	}

	timestamp, err := parseFloat(timestampBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse timestamp: '%s' (%s)", input, err)
	}

	parsedMetric := &ParsedMetric{
		moira2.UnsafeBytesToString(metricBytes),
		name,
		labels,
		value,
		int64(timestamp),
	}
	if timestamp == -1 {
		parsedMetric.Timestamp = time.Now().Unix()
	}
	return parsedMetric, nil
}

func parseNameAndLabels(metricBytes []byte) (string, map[string]string, error) {
	metricBytesScanner := moira2.NewBytesScanner(metricBytes, ';')
	if !metricBytesScanner.HasNext() {
		return "", nil, fmt.Errorf("too few colon-separated items: '%s'", metricBytes)
	}
	nameBytes := metricBytesScanner.Next()
	if len(nameBytes) == 0 {
		return "", nil, fmt.Errorf("empty metric name: '%s'", metricBytes)
	}
	name := moira2.UnsafeBytesToString(nameBytes)
	labels := make(map[string]string)
	for metricBytesScanner.HasNext() {
		labelBytes := metricBytesScanner.Next()
		labelBytesScanner := moira2.NewBytesScanner(labelBytes, '=')

		var labelNameBytes, labelValueBytes []byte
		if !labelBytesScanner.HasNext() {
			return "", nil, fmt.Errorf("too few equal-separated items: '%s'", labelBytes)
		}
		labelNameBytes = labelBytesScanner.Next()
		if !labelBytesScanner.HasNext() {
			return "", nil, fmt.Errorf("too few equal-separated items: '%s'", labelBytes)
		}
		labelValueBytes = labelBytesScanner.Next()
		for labelBytesScanner.HasNext() {
			var labelString strings.Builder
			labelString.WriteString("=")
			labelString.Write(labelBytesScanner.Next())
			labelValueBytes = append(labelValueBytes, labelString.String()...)
		}
		if len(labelNameBytes) == 0 {
			return "", nil, fmt.Errorf("empty label name: '%s'", labelBytes)
		}
		labelName := moira2.UnsafeBytesToString(labelNameBytes)
		labelValue := moira2.UnsafeBytesToString(labelValueBytes)
		labels[labelName] = labelValue
	}
	return name, labels, nil
}

func parseFloat(input []byte) (float64, error) {
	return strconv.ParseFloat(moira2.UnsafeBytesToString(input), 64)
}

func isPrintableASCII(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] < 0x20 || b[i] > 0x7E {
			return false
		}
	}

	return true
}