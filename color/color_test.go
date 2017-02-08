package color

import (
	"fmt"
	"testing"
)

func TestColors(t *testing.T) {
	ColorFlag = true
	colorFns := []Colorfn{Black, Red, Green, Yellow, Magenta, Cyan, White, DefaultColor}
	colorCodes := []Color{Blackfg, Redfg, Greenfg, Yellowfg, Magentafg, Cyanfg, Whitefg, Defaultfg}

	// Test with attr
	for i, colorCode := range colorCodes {
		fn := colorFns[i]
		str := fmt.Sprintf("test string: %d", i)
		coloredStr := fn("test string: %d", i)
		expectedStr := fmt.Sprintf("%v%vm%v%v", escape, colorCode, str, clear)
		if colorCode == Defaultfg {
			expectedStr = str
		}
		fmt.Println(coloredStr)
		if coloredStr != expectedStr {
			t.Fatalf("Expected string:%v\n, got: %v", expectedStr, coloredStr)

		}
	}

	// Test without attr
	for i, colorCode := range colorCodes {
		fn := colorFns[i]
		str := "test string"
		coloredStr := fn(str)
		expectedStr := fmt.Sprintf("%v%vm%v%v", escape, colorCode, str, clear)
		if colorCode == Defaultfg {
			expectedStr = str
		} else {
		}
		if coloredStr != expectedStr {
			t.Fatalf("Expected string:%v\n, got: %v", expectedStr, coloredStr)

		}
	}
}

func TestColorsDisabled(t *testing.T) {
	ColorFlag = false
	colorFns := []Colorfn{Black, Red, Green, Yellow, Magenta, Cyan, White, DefaultColor}

	// Test with attr
	for i, fn := range colorFns {
		str := fmt.Sprintf("test string: %d", i)
		coloredStr := fn("test string: %d", i)
		if coloredStr != str {
			t.Fatalf("Expected string:%v\n, got: %v", str, coloredStr)

		}
	}

	// Test without attr
	for _, fn := range colorFns {
		str := "test string"
		coloredStr := fn(str)
		if coloredStr != str {
			t.Fatalf("Expected string:%v\n, got: %v", str, coloredStr)

		}
	}
}
