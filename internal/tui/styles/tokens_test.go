package styles

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

const nanInkThemeAssetPath = "../../assets/themes/cortex-ia.json"

var nanInkBaseHues = []string{"gray", "red", "orange", "yellow", "green", "cyan", "blue", "purple"}

var nanInkHueSteps = []string{"100", "200", "300", "400", "500", "600", "700", "800", "900"}

var nanInkTextRoleKeys = []string{"Text", "Body", "Muted", "AccentText", "Success", "Warning", "Danger"}

func nanInkAllRoles(t NanInkTokens) map[string]string {
	return map[string]string{
		"BG": string(t.BG), "Surface": string(t.Surface), "Line": string(t.Line),
		"Text": string(t.Text), "Body": string(t.Body), "Muted": string(t.Muted),
		"AccentBorder": string(t.AccentBorder), "AccentText": string(t.AccentText),
		"Selected": string(t.Selected), "Success": string(t.Success),
		"Warning": string(t.Warning), "Danger": string(t.Danger),
	}
}

func nanInkLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	channel := func(pair string) float64 {
		n, err := strconv.ParseInt(pair, 16, 32)
		if err != nil {
			panic(err)
		}
		c := float64(n) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(hex[0:2]) + 0.7152*channel(hex[2:4]) + 0.0722*channel(hex[4:6])
}

func nanInkContrast(fg, bg string) float64 {
	a, b := nanInkLuminance(fg), nanInkLuminance(bg)
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

func TestNanInkTokensPinned(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		got, want NanInkTokens
	}{
		{"dark", NanInkDark, NanInkTokens{
			BG: "#0B0B0C", Surface: "#121214", Line: "#3A3A3D",
			Text: "#FFFFFF", Body: "#C9C9CC", Muted: "#9A9A9E",
			AccentBorder: "#7D39EB", AccentText: "#9B6BF0", Selected: "#1D1430",
			Success: "#22C55E", Warning: "#F59E0B", Danger: "#EF4444",
		}},
		{"light", NanInkLight, NanInkTokens{
			BG: "#EFEEEB", Surface: "#E3E1DC", Line: "#D5D3CE",
			Text: "#101010", Body: "#2E2E2E", Muted: "#6F6F6F",
			AccentBorder: "#7D39EB", AccentText: "#7D39EB", Selected: "#E7DCFB",
			Success: "#166534", Warning: "#92400E", Danger: "#B91C1C",
		}},
	} {
		if tc.got != tc.want {
			t.Errorf("NanInk %s drifted from the pinned palette:\n got %+v\nwant %+v", tc.mode, tc.got, tc.want)
		}
	}
}

func TestNanInkAccentTextMeetsAA(t *testing.T) {
	got := nanInkContrast(string(NanInkDark.AccentText), string(NanInkDark.BG))
	if got < 4.5 {
		t.Errorf("dark accent-text %s on %s is %.2f:1, below AA 4.5:1", NanInkDark.AccentText, NanInkDark.BG, got)
	}
	if rounded := math.Round(got*100) / 100; rounded != 5.39 {
		t.Errorf("dark accent-text contrast is %.2f:1, want the pinned 5.39:1", got)
	}
	if light := nanInkContrast(string(NanInkLight.AccentText), string(NanInkLight.BG)); light < 4.5 {
		t.Errorf("light accent-text %s on %s is %.2f:1, below AA 4.5:1", NanInkLight.AccentText, NanInkLight.BG, light)
	}
}

func TestNanInkAccentBorderIsDecorationOnly(t *testing.T) {
	got := nanInkContrast(string(NanInkDark.AccentBorder), string(NanInkDark.BG))
	if got >= 4.5 {
		t.Errorf("dark accent-border %s on %s is %.2f:1; it must stay below AA so it is never usable as text", NanInkDark.AccentBorder, NanInkDark.BG, got)
	}
	roles := nanInkAllRoles(NanInkDark)
	for _, role := range nanInkTextRoleKeys {
		if roles[role] == string(NanInkDark.AccentBorder) {
			t.Errorf("accent-border %s was promoted into the dark text role %s", NanInkDark.AccentBorder, role)
		}
	}
}

func nanInkLoadMode(raw json.RawMessage) (map[string]map[string]string, map[string]string, error) {
	var top struct {
		Hue map[string]json.RawMessage `json:"hue"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, nil, err
	}
	scales := make(map[string]map[string]string)
	aliases := make(map[string]string)
	for name, rawScale := range top.Hue {
		var ref string
		if err := json.Unmarshal(rawScale, &ref); err == nil && strings.HasPrefix(ref, "$hue.") {
			aliases[name] = strings.TrimPrefix(ref, "$hue.")
			continue
		}
		var scale map[string]string
		if err := json.Unmarshal(rawScale, &scale); err != nil {
			return nil, nil, err
		}
		scales[name] = scale
	}
	return scales, aliases, nil
}

func nanInkResolve(ref string, scales map[string]map[string]string, aliases map[string]string) (string, bool) {
	rest, ok := strings.CutPrefix(ref, "$hue.")
	if !ok {
		return "", false
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 2 {
		return "", false
	}
	name := parts[0]
	if target, ok := aliases[name]; ok {
		name = target
	}
	scale, ok := scales[name]
	if !ok {
		return "", false
	}
	value, ok := scale[parts[1]]
	return value, ok
}

func nanInkHexSet(scales map[string]map[string]string) map[string]bool {
	set := make(map[string]bool)
	for _, scale := range scales {
		for _, value := range scale {
			set[value] = true
		}
	}
	return set
}

type nanInkBaseTree struct {
	Categorical []string `json:"categorical"`
	Text        struct {
		Base   string
		Muted  string
		Action struct {
			Primary struct{ Base string }
		}
	}
	Background struct {
		Base   string
		Raised struct{ Base string }
	}
	Border struct{ Base string }
}

func TestNanInkThemeAssetMirrorsTokens(t *testing.T) {
	data, err := os.ReadFile(nanInkThemeAssetPath)
	if err != nil {
		t.Fatalf("read theme asset: %v", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("theme asset is not valid JSON: %v", err)
	}
	baseRaw, ok := root["base"]
	if !ok {
		t.Fatal("theme asset is missing the required 'base' token tree")
	}
	var base nanInkBaseTree
	if err := json.Unmarshal(baseRaw, &base); err != nil {
		t.Fatalf("parse theme base tree: %v", err)
	}
	if len(base.Categorical) == 0 {
		t.Error("theme base.categorical must not be empty")
	}

	type palette struct {
		scales  map[string]map[string]string
		aliases map[string]string
	}
	palettes := make(map[string]palette, 2)
	tokens := map[string]NanInkTokens{"dark": NanInkDark, "light": NanInkLight}
	for mode, want := range tokens {
		scales, aliases, err := nanInkLoadMode(root[mode])
		if err != nil {
			t.Fatalf("parse %s mode: %v", mode, err)
		}
		palettes[mode] = palette{scales, aliases}
		carried := nanInkHexSet(scales)
		for _, hue := range nanInkBaseHues {
			scale, ok := scales[hue]
			if !ok {
				t.Errorf("%s mode is missing the %q hue scale", mode, hue)
				continue
			}
			for _, step := range nanInkHueSteps {
				if value := scale[step]; len(value) != 7 || value[0] != '#' {
					t.Errorf("%s hue %s.%s = %q is not a literal #RRGGBB hex", mode, hue, step, value)
				}
			}
		}
		for role, hex := range nanInkAllRoles(want) {
			if !carried[hex] {
				t.Errorf("%s mode hue scales do not carry canonical %s token %s", mode, role, hex)
			}
		}
	}

	for _, tc := range []struct {
		name, ref, dark, light string
	}{
		{"text.base", base.Text.Base, string(NanInkDark.Text), string(NanInkLight.Text)},
		{"text.muted", base.Text.Muted, string(NanInkDark.Muted), string(NanInkLight.Muted)},
		{"text.action.primary.base", base.Text.Action.Primary.Base, string(NanInkDark.AccentText), string(NanInkLight.AccentText)},
		{"background.base", base.Background.Base, string(NanInkDark.BG), string(NanInkLight.BG)},
		{"background.raised.base", base.Background.Raised.Base, string(NanInkDark.Surface), string(NanInkLight.Surface)},
		{"border.base", base.Border.Base, string(NanInkDark.Line), string(NanInkLight.Line)},
	} {
		for mode, want := range map[string]string{"dark": tc.dark, "light": tc.light} {
			p := palettes[mode]
			got, ok := nanInkResolve(tc.ref, p.scales, p.aliases)
			if !ok {
				t.Errorf("%s.%s reference %q did not resolve", mode, tc.name, tc.ref)
				continue
			}
			if got != want {
				t.Errorf("%s.%s = %s, want canonical token %s", mode, tc.name, got, want)
			}
		}
	}
}
