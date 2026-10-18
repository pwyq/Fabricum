package processing

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type svgPathSegment struct {
	command byte
	numbers []string
	values  []float64
}

type svgPathReader struct {
	data  string
	i     int
	comma bool
}

func svgPathArity(command byte) int {
	switch command | 32 {
	case 'm', 'l', 't':
		return 2
	case 'h', 'v':
		return 1
	case 'c':
		return 6
	case 's', 'q':
		return 4
	case 'a':
		return 7
	case 'z':
		return 0
	}
	return -1
}

func (r *svgPathReader) space() {
	for r.i < len(r.data) && svgSpace(r.data[r.i]) {
		r.i++
	}
}

func (r *svgPathReader) number(flag bool) (string, float64, error) {
	r.space()
	if r.i < len(r.data) && r.data[r.i] == ',' && r.comma {
		r.i++
		r.space()
	}
	start := r.i
	if flag {
		if r.i < len(r.data) && (r.data[r.i] == '0' || r.data[r.i] == '1') {
			r.i++
		}
	} else {
		if r.i < len(r.data) && (r.data[r.i] == '-' || r.data[r.i] == '+') {
			r.i++
		}
		digits := 0
		for r.i < len(r.data) && r.data[r.i] >= '0' && r.data[r.i] <= '9' {
			r.i++
			digits++
		}
		if r.i < len(r.data) && r.data[r.i] == '.' {
			r.i++
			for r.i < len(r.data) && r.data[r.i] >= '0' && r.data[r.i] <= '9' {
				r.i++
				digits++
			}
		}
		if digits == 0 {
			return "", 0, fmt.Errorf("expected path number at byte %d", start)
		}
		if r.i < len(r.data) && (r.data[r.i] == 'e' || r.data[r.i] == 'E') {
			r.i++
			if r.i < len(r.data) && (r.data[r.i] == '-' || r.data[r.i] == '+') {
				r.i++
			}
			for r.i < len(r.data) && r.data[r.i] >= '0' && r.data[r.i] <= '9' {
				r.i++
			}
		}
	}
	token := r.data[start:r.i]
	value, err := strconv.ParseFloat(token, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return "", 0, fmt.Errorf("invalid path number at byte %d", start)
	}
	r.comma = true
	return token, value, nil
}

func parseSVGPath(data string) ([]svgPathSegment, error) {
	r := svgPathReader{data: data}
	var segments []svgPathSegment
	var command byte
	for {
		r.space()
		if r.i == len(data) {
			return segments, nil
		}
		if svgPathArity(data[r.i]) >= 0 {
			command = data[r.i]
			r.i++
			r.comma = false
		}
		if command == 0 || len(segments) == 0 && command|32 != 'm' {
			return nil, fmt.Errorf("SVG path must start with moveto and use valid commands")
		}
		segment := svgPathSegment{command: command}
		for i := 0; i < svgPathArity(command); i++ {
			token, value, err := r.number(command|32 == 'a' && (i == 3 || i == 4))
			if err != nil {
				return nil, err
			}
			if command|32 == 'a' && i < 2 && value < 0 {
				return nil, fmt.Errorf("SVG arc radii must be nonnegative")
			}
			segment.numbers = append(segment.numbers, token)
			segment.values = append(segment.values, value)
		}
		segments = append(segments, segment)
		if len(segments) > 200000 {
			return nil, fmt.Errorf("SVG path exceeds segment limit")
		}
		switch command {
		case 'm':
			command = 'l'
		case 'M':
			command = 'L'
		case 'z', 'Z':
			command = 0
		}
	}
}

// Minification changes spelling only. In particular it does not round long
// decimals through float64 when no precision option was requested.
func compactSVGNumber(token string) string {
	token = strings.TrimPrefix(token, "+")
	if strings.ContainsAny(token, "eE") {
		return token
	}
	sign := ""
	if strings.HasPrefix(token, "-") {
		sign, token = "-", token[1:]
	}
	if strings.Contains(token, ".") {
		token = strings.TrimRight(strings.TrimRight(token, "0"), ".")
	}
	token = strings.TrimLeft(token, "0")
	if token == "" {
		return "0"
	}
	return sign + token
}

func writeSVGPath(segments []svgPathSegment) string {
	var output strings.Builder
	var previous byte
	for _, segment := range segments {
		command := segment.command
		if command != previous || command|32 == 'm' || command|32 == 'z' {
			output.WriteByte(command)
		}
		for _, token := range segment.numbers {
			token = compactSVGNumber(token)
			// A minus sign delimits numbers except inside an exponent. Other
			// adjacent numeric tokens always receive an unambiguous separator.
			last := output.String()[output.Len()-1]
			if (last >= '0' && last <= '9' || last == '.') && token[0] != '-' {
				output.WriteByte(' ')
			}
			output.WriteString(token)
		}
		previous = command
	}
	return output.String()
}
