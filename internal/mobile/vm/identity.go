// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Identity is the hardware and build profile a mobile device presents to
// Android apps so they see a consistent, realistic phone instead of a generic
// LineageOS QEMU build.
type Identity struct {
	Template     string `json:"template"`
	Model        string `json:"model"`
	Brand        string `json:"brand"`
	Manufacturer string `json:"manufacturer"`
	Device       string `json:"device"`
	Product      string `json:"product"`
	Board        string `json:"board"`
	Platform     string `json:"platform"`   // ro.board.platform (hides virtio/ranchu)
	Hardware     string `json:"hardware"`   // ro.hardware & ro.boot.hardware
	Baseband     string `json:"baseband"`   // ro.boot.baseband (hides unknown)
	Fingerprint  string `json:"fingerprint"`
	BuildID      string `json:"buildId"`
	Incremental  string `json:"incremental"`
	SecurityDate string `json:"securityDate"`
	MIDR         string `json:"midr"`       // QEMU -cpu host,midr=<MIDR> (hides Apple Silicon 0x61)
	Serial       string `json:"serial"`     // unique per device
	IMEI         string `json:"imei"`       // Luhn-valid 15-digit IMEI
	TAC          string `json:"tac"`        // 8-digit Type Allocation Code for the template
	GLVendor     string `json:"glVendor"`   // Mesa driconf force_gl_vendor
	GLRenderer   string `json:"glRenderer"` // Mesa driconf force_gl_renderer
}

// Carrier holds SIM/network operator properties matching a proxy's country.
type Carrier struct {
	Alpha   string
	Numeric string
	ISO     string
}

var identityTemplates = []Identity{
	{
		Template:     "pixel8",
		Model:        "Pixel 8",
		Brand:        "google",
		Manufacturer: "Google",
		Device:       "shiba",
		Product:      "shiba",
		Board:        "shiba",
		Hardware:     "shiba",
		Fingerprint:  "google/shiba/shiba:16/BP2A.250605.031.A2/13578691:user/release-keys",
		BuildID:      "BP2A.250605.031.A2",
		Incremental:  "13578691",
		SecurityDate: "2025-06-05",
		MIDR:         "0x411fd410", // ARM Cortex-A78
		TAC:          "35391811",
		GLVendor:     "ARM",
		GLRenderer:   "Mali-G715",
	},
	{
		Template:     "pixel9",
		Model:        "Pixel 9",
		Brand:        "google",
		Manufacturer: "Google",
		Device:       "tokay",
		Product:      "tokay",
		Board:        "tokay",
		Hardware:     "tokay",
		Fingerprint:  "google/tokay/tokay:16/BP2A.250605.031.A2/13578691:user/release-keys",
		BuildID:      "BP2A.250605.031.A2",
		Incremental:  "13578691",
		SecurityDate: "2025-06-05",
		MIDR:         "0x411fd410",
		TAC:          "35482912",
		GLVendor:     "ARM",
		GLRenderer:   "Mali-G715",
	},
	{
		Template:     "s24",
		Model:        "SM-S921B",
		Brand:        "samsung",
		Manufacturer: "samsung",
		Device:       "e1s",
		Product:      "e1sxxx",
		Board:        "s5e9945",
		Hardware:     "s5e9945",
		Fingerprint:  "samsung/e1sxxx/e1s:15/AP3A.240905.015.A2/S921BXXU4BYD9:user/release-keys",
		BuildID:      "AP3A.240905.015.A2",
		Incremental:  "S921BXXU4BYD9",
		SecurityDate: "2025-05-01",
		MIDR:         "0x411fd410",
		TAC:          "35046981",
		GLVendor:     "Qualcomm",
		GLRenderer:   "Adreno (TM) 750",
	},
	{
		Template:     "oneplus12",
		Model:        "CPH2581",
		Brand:        "OnePlus",
		Manufacturer: "OnePlus",
		Device:       "OP5929L1",
		Product:      "CPH2581EEA",
		Board:        "pineapple",
		Hardware:     "qcom",
		Fingerprint:  "OnePlus/CPH2581EEA/OP5929L1:15/AP3A.240617.008/U.1b7f23a_1-1:user/release-keys",
		BuildID:      "AP3A.240617.008",
		Incremental:  "U.1b7f23a_1-1",
		SecurityDate: "2025-05-01",
		MIDR:         "0x411fd410",
		TAC:          "86492106",
		GLVendor:     "Qualcomm",
		GLRenderer:   "Adreno (TM) 750",
	},
	{
		Template:     "s24ultra",
		Model:        "SM-S928B",
		Brand:        "samsung",
		Manufacturer: "samsung",
		Device:       "e3q",
		Product:      "e3qxxx",
		Board:        "pineapple",
		Hardware:     "qcom",
		Fingerprint:  "samsung/e3qxxx/e3q:15/AP3A.240905.015.A2/S928BXXU4BYD9:user/release-keys",
		BuildID:      "AP3A.240905.015.A2",
		Incremental:  "S928BXXU4BYD9",
		SecurityDate: "2025-05-01",
		MIDR:         "0x411fd410",
		TAC:          "35089412",
		GLVendor:     "Qualcomm",
		GLRenderer:   "Adreno (TM) 750",
	},
	{
		Template:     "s23ultra",
		Model:        "SM-S918B",
		Brand:        "samsung",
		Manufacturer: "samsung",
		Device:       "dm3q",
		Product:      "dm3qxxx",
		Board:        "kalama",
		Hardware:     "qcom",
		Fingerprint:  "samsung/dm3qxxx/dm3q:15/AP3A.240905.015.A2/S918BXXU8DYD9:user/release-keys",
		BuildID:      "AP3A.240905.015.A2",
		Incremental:  "S918BXXU8DYD9",
		SecurityDate: "2025-05-01",
		MIDR:         "0x411fd410",
		TAC:          "35015789",
		GLVendor:     "Qualcomm",
		GLRenderer:   "Adreno (TM) 740",
	},
	{
		Template:     "a54",
		Model:        "SM-A546E",
		Brand:        "samsung",
		Manufacturer: "samsung",
		Device:       "a54x",
		Product:      "a54xnsxx",
		Board:        "s5e8835",
		Hardware:     "s5e8835",
		Fingerprint:  "samsung/a54xnsxx/a54x:15/AP3A.240905.015.A2/A546EXXU9CYD1:user/release-keys",
		BuildID:      "AP3A.240905.015.A2",
		Incremental:  "A546EXXU9CYD1",
		SecurityDate: "2025-05-01",
		MIDR:         "0x411fd410",
		TAC:          "35629104",
		GLVendor:     "ARM",
		GLRenderer:   "Mali-G68",
	},
	{
		Template:     "xiaomi14pro",
		Model:        "23116PN5BC",
		Brand:        "Xiaomi",
		Manufacturer: "Xiaomi",
		Device:       "shennong",
		Product:      "shennong",
		Board:        "pineapple",
		Hardware:     "qcom",
		Fingerprint:  "Xiaomi/shennong/shennong:15/UKQ1.230804.001/OS2.0.105.0.VNBCNXM:user/release-keys",
		BuildID:      "UKQ1.230804.001",
		Incremental:  "OS2.0.105.0.VNBCNXM",
		SecurityDate: "2025-04-01",
		MIDR:         "0x411fd410",
		TAC:          "86789105",
		GLVendor:     "Qualcomm",
		GLRenderer:   "Adreno (TM) 750",
	},
	{
		Template:     "xiaomi13ultra",
		Model:        "2304FPN6DG",
		Brand:        "Xiaomi",
		Manufacturer: "Xiaomi",
		Device:       "ishtar",
		Product:      "ishtar_global",
		Board:        "kalama",
		Hardware:     "qcom",
		Fingerprint:  "Xiaomi/ishtar_global/ishtar:15/UKQ1.230804.001/OS2.0.102.0.VMAMIXM:user/release-keys",
		BuildID:      "UKQ1.230804.001",
		Incremental:  "OS2.0.102.0.VMAMIXM",
		SecurityDate: "2025-04-01",
		MIDR:         "0x411fd410",
		TAC:          "86521406",
		GLVendor:     "Qualcomm",
		GLRenderer:   "Adreno (TM) 740",
	},
	{
		Template:     "oppofindx6pro",
		Model:        "PGEM10",
		Brand:        "OPPO",
		Manufacturer: "OPPO",
		Device:       "OP5553L1",
		Product:      "PGEM10",
		Board:        "kalama",
		Hardware:     "qcom",
		Fingerprint:  "OPPO/PGEM10/OP5553L1:15/UKQ1.230924.001/T.19e84a2-1:user/release-keys",
		BuildID:      "UKQ1.230924.001",
		Incremental:  "T.19e84a2-1",
		SecurityDate: "2025-04-01",
		MIDR:         "0x411fd410",
		TAC:          "86394205",
		GLVendor:     "Qualcomm",
		GLRenderer:   "Adreno (TM) 740",
	},
}

var countryCarriers = map[string]Carrier{
	"VN": {Alpha: "Viettel", Numeric: "45204", ISO: "vn"},
	"US": {Alpha: "T-Mobile", Numeric: "310260", ISO: "us"},
	"GB": {Alpha: "Vodafone UK", Numeric: "23415", ISO: "gb"},
	"DE": {Alpha: "Telekom.de", Numeric: "26201", ISO: "de"},
	"FR": {Alpha: "Orange F", Numeric: "20801", ISO: "fr"},
	"JP": {Alpha: "NTT DOCOMO", Numeric: "44010", ISO: "jp"},
	"KR": {Alpha: "SKTelecom", Numeric: "45005", ISO: "kr"},
	"SG": {Alpha: "Singtel", Numeric: "52501", ISO: "sg"},
	"TH": {Alpha: "AIS", Numeric: "52003", ISO: "th"},
	"ID": {Alpha: "Telkomsel", Numeric: "51010", ISO: "id"},
	"AU": {Alpha: "Telstra", Numeric: "50501", ISO: "au"},
	"CA": {Alpha: "Rogers", Numeric: "302720", ISO: "ca"},
	"MY": {Alpha: "Maxis", Numeric: "50212", ISO: "my"},
	"PH": {Alpha: "Globe", Numeric: "51502", ISO: "ph"},
	"BR": {Alpha: "Vivo", Numeric: "72410", ISO: "br"},
}

// IdentityTemplates returns the built-in mobile hardware template IDs.
func IdentityTemplates() []string {
	out := make([]string, len(identityTemplates))
	for i, tpl := range identityTemplates {
		out[i] = tpl.Template
	}
	return out
}

// PickDistinctTemplate chooses a template from candidates deterministically
// from seed while strongly preferring templates with the lowest count in used.
func PickDistinctTemplate(seed string, candidates []string, used map[string]int) string {
	if len(candidates) == 0 {
		return ""
	}
	minUse := -1
	for _, c := range candidates {
		n := used[c]
		if minUse < 0 || n < minUse {
			minUse = n
		}
	}
	pool := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if used[c] == minUse {
			pool = append(pool, c)
		}
	}
	h := sha256.Sum256([]byte("anyprofile-mobile-pick:" + seed))
	idx := (int(h[0])<<8 | int(h[1])) % len(pool)
	return pool[idx]
}

// IdentityFromProfile constructs a valid device Identity from an external
// hardware profile (such as an AVO device JSON) with deterministic Serial/IMEI.
func IdentityFromProfile(deviceID, templateID string, props map[string]string, board, platform, hardware, glVendor, glRenderer string) (Identity, bool) {
	model := strings.TrimSpace(props["ro.product.model"])
	brand := strings.TrimSpace(props["ro.product.brand"])
	if model == "" || brand == "" {
		return Identity{}, false
	}
	mfr := strings.TrimSpace(props["ro.product.manufacturer"])
	if mfr == "" {
		mfr = brand
	}
	dev := strings.TrimSpace(props["ro.product.device"])
	if dev == "" {
		dev = strings.ToLower(strings.ReplaceAll(model, " ", "_"))
	}
	prod := strings.TrimSpace(props["ro.product.name"])
	if prod == "" {
		prod = dev
	}
	fp := strings.TrimSpace(props["ro.build.fingerprint"])
	buildID := strings.TrimSpace(props["ro.build.id"])
	inc := strings.TrimSpace(props["ro.build.version.incremental"])
	if fp != "" && (buildID == "" || inc == "") {
		// Format: brand/product/device:release/buildID/incremental:type/tags
		if parts := strings.Split(fp, "/"); len(parts) >= 5 {
			if buildID == "" {
				buildID = parts[3]
			}
			if inc == "" {
				inc = strings.SplitN(parts[4], ":", 2)[0]
			}
		}
	}
	if buildID == "" {
		buildID = "AP3A.240905.015.A2"
	}
	if inc == "" {
		inc = "13578691"
	}
	if fp == "" {
		fp = fmt.Sprintf("%s/%s/%s:15/%s/%s:user/release-keys", brand, prod, dev, buildID, inc)
	}
	if board == "" {
		board = dev
	}
	if platform == "" {
		platform = board
	}
	if hardware == "" {
		hardware = platform
	}
	tac := tacForBrand(brand)
	h := sha256.Sum256([]byte("anyprofile-mobile-identity:" + deviceID))
	id := Identity{
		Template:     templateID,
		Model:        model,
		Brand:        brand,
		Manufacturer: mfr,
		Device:       dev,
		Product:      prod,
		Board:        board,
		Platform:     platform,
		Hardware:     hardware,
		Baseband:     "msm",
		Fingerprint:  fp,
		BuildID:      buildID,
		Incremental:  inc,
		SecurityDate: "2025-05-01",
		MIDR:         "0x411fd410",
		Serial:       strings.ToUpper(hex.EncodeToString(h[1:8])),
		IMEI:         luhnIMEI(tac, h[8:14]),
		TAC:          tac,
		GLVendor:     glVendor,
		GLRenderer:   glRenderer,
	}
	if strings.EqualFold(brand, "google") {
		id.Baseband = "g5300q-240919-241106-B-12612894"
	}
	return id, true
}

func tacForBrand(brand string) string {
	switch strings.ToLower(strings.TrimSpace(brand)) {
	case "google":
		return "35391811"
	case "samsung":
		return "35089412"
	case "xiaomi", "redmi", "poco":
		return "86789105"
	case "oneplus":
		return "86492106"
	case "oppo", "realme":
		return "86394205"
	case "vivo":
		return "86612405"
	case "asus":
		return "35762911"
	case "nothing":
		return "35841209"
	default:
		return "35482912"
	}
}

// IdentityFor returns a deterministic device identity for deviceID. If template
// is empty or unknown, a template is chosen deterministically from deviceID.
func IdentityFor(deviceID, template string) Identity {
	h := sha256.Sum256([]byte("anyprofile-mobile-identity:" + deviceID))
	var chosen Identity
	for _, tpl := range identityTemplates {
		if tpl.Template == template {
			chosen = tpl
			break
		}
	}
	if chosen.Template == "" {
		chosen = identityTemplates[int(h[0])%len(identityTemplates)]
	}
	// Realistic 14-char uppercase alphanumeric serial number.
	chosen.Serial = strings.ToUpper(hex.EncodeToString(h[1:8]))
	chosen.IMEI = luhnIMEI(chosen.TAC, h[8:14])
	if chosen.Platform == "" {
		if chosen.Brand == "google" {
			chosen.Platform = "zuma"
		} else {
			chosen.Platform = chosen.Board
		}
	}
	if chosen.Baseband == "" {
		if chosen.Brand == "google" {
			chosen.Baseband = "g5300q-240919-241106-B-12612894"
		} else {
			chosen.Baseband = "msm"
		}
	}
	return chosen
}

// luhnIMEI builds a Luhn-valid 15-digit IMEI from an 8-digit TAC and 6 seed bytes.
func luhnIMEI(tac string, seed []byte) string {
	if len(tac) != 8 {
		tac = "35391811"
	}
	var digits [14]int
	for i := 0; i < 8; i++ {
		digits[i] = int(tac[i] - '0')
	}
	for i := 0; i < 6; i++ {
		digits[8+i] = int(seed[i]) % 10
	}
	sum := 0
	for i, d := range digits {
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	check := (10 - (sum % 10)) % 10
	var b strings.Builder
	b.Grow(15)
	for _, d := range digits {
		b.WriteByte(byte('0' + d))
	}
	b.WriteByte(byte('0' + check))
	return b.String()
}

// ValidIMEI checks that imei is 15 ASCII digits and passes the Luhn checksum.
func ValidIMEI(imei string) bool {
	if len(imei) != 15 {
		return false
	}
	sum := 0
	for i := 0; i < 15; i++ {
		if imei[i] < '0' || imei[i] > '9' {
			return false
		}
		d := int(imei[i] - '0')
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// ParseIdentity decodes a stored IdentityJSON; an empty string returns zero and false.
func ParseIdentity(raw string) (Identity, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Identity{}, false
	}
	var id Identity
	if err := json.Unmarshal([]byte(raw), &id); err != nil || id.Model == "" {
		return Identity{}, false
	}
	return id, true
}

// MarshalIdentity serializes an Identity to JSON.
func MarshalIdentity(id Identity) string {
	b, _ := json.Marshal(id)
	return string(b)
}

// CarrierFor returns a realistic carrier for an ISO-3166 alpha-2 country code.
func CarrierFor(country string) (Carrier, bool) {
	c, ok := countryCarriers[strings.ToUpper(strings.TrimSpace(country))]
	return c, ok
}

func validPropValue(v string) bool {
	if v == "" || len(v) > 200 {
		return false
	}
	for _, r := range v {
		if r < 0x20 || r > 0x7e || r == '\'' || r == '"' || r == '\\' || r == '$' || r == '`' || r == ';' || r == '|' || r == '&' {
			return false
		}
	}
	return true
}

// RenderIdentityScript renders the Magisk post-fs-data script that spoofs
// Android system properties before app processes start.
func RenderIdentityScript(id Identity) (string, error) {
	props := [][2]string{
		{"ro.product.model", id.Model},
		{"ro.product.brand", id.Brand},
		{"ro.product.manufacturer", id.Manufacturer},
		{"ro.product.device", id.Device},
		{"ro.product.name", id.Product},
		{"ro.product.board", id.Board},
		{"ro.product.system.model", id.Model},
		{"ro.product.system.brand", id.Brand},
		{"ro.product.system.manufacturer", id.Manufacturer},
		{"ro.product.system.device", id.Device},
		{"ro.product.system.name", id.Product},
		{"ro.product.vendor.model", id.Model},
		{"ro.product.vendor.brand", id.Brand},
		{"ro.product.vendor.manufacturer", id.Manufacturer},
		{"ro.product.vendor.device", id.Device},
		{"ro.product.vendor.name", id.Product},
		{"ro.build.fingerprint", id.Fingerprint},
		{"ro.system.build.fingerprint", id.Fingerprint},
		{"ro.vendor.build.fingerprint", id.Fingerprint},
		{"ro.bootimage.build.fingerprint", id.Fingerprint},
		{"ro.build.id", id.BuildID},
		{"ro.build.version.incremental", id.Incremental},
		{"ro.build.version.security_patch", id.SecurityDate},
		{"ro.build.tags", "release-keys"},
		{"ro.build.type", "user"},
		{"ro.debuggable", "0"},
		{"ro.secure", "1"},
		{"ro.boot.verifiedbootstate", "green"},
		{"ro.boot.flash.locked", "1"},
		{"ro.boot.vbmeta.device_state", "locked"},
		{"ro.serialno", id.Serial},
		{"ro.boot.serialno", id.Serial},
		{"ro.hardware", id.Hardware},
		{"ro.boot.hardware", id.Hardware},
		{"ro.board.platform", id.Platform},
		{"ro.boot.baseband", id.Baseband},
		{"ro.build.product", id.Product},
		{"ro.boot.mode", "normal"},
		{"ro.kernel.qemu", "0"},
		{"ro.boot.qemu", "0"},
	}
	var b strings.Builder
	b.WriteString("#!/system/bin/sh\n")
	b.WriteString("# AnyProfile per-device identity (applied in post-fs-data)\n")
	b.WriteString("RESETPROP=$(command -v resetprop || echo /data/adb/magisk/magisk)\n")
	b.WriteString("rp() {\n")
	b.WriteString("  if [ \"$RESETPROP\" = \"/data/adb/magisk/magisk\" ]; then\n")
	b.WriteString("    /data/adb/magisk/magisk resetprop \"$@\"\n")
	b.WriteString("  else\n")
	b.WriteString("    \"$RESETPROP\" \"$@\"\n")
	b.WriteString("  fi\n")
	b.WriteString("}\n")
	for _, kv := range props {
		if !validPropValue(kv[1]) {
			return "", fmt.Errorf("invalid identity property %s=%q", kv[0], kv[1])
		}
		fmt.Fprintf(&b, "rp '%s' '%s'\n", kv[0], kv[1])
	}
	// Strip LineageOS-specific props so apps do not see a custom ROM signature.
	b.WriteString("for p in $(getprop | sed -n 's/^\\[\\(ro\\.lineage\\.[^]]*\\)\\].*/\\1/p'); do\n")
	b.WriteString("  rp --delete \"$p\"\n")
	b.WriteString("done\n")
	return b.String(), nil
}

// CarrierShellScript returns a root shell command applying carrier/SIM props
// for the given ISO country code. Returns "" when country has no mapped carrier.
func CarrierShellScript(country string) string {
	c, ok := CarrierFor(country)
	if !ok {
		return ""
	}
	if !validPropValue(c.Alpha) || !validPropValue(c.Numeric) || !validPropValue(c.ISO) {
		return ""
	}
	return fmt.Sprintf(
		"su -c \"resetprop gsm.operator.alpha '%s'; resetprop gsm.sim.operator.alpha '%s'; resetprop gsm.operator.numeric '%s'; resetprop gsm.sim.operator.numeric '%s'; resetprop gsm.operator.iso-country '%s'; resetprop gsm.sim.operator.iso-country '%s'; resetprop gsm.sim.state READY\"",
		c.Alpha, c.Alpha, c.Numeric, c.Numeric, c.ISO, c.ISO,
	)
}

// BatteryShellScript returns an adb shell command that sets a realistic
// unplugged battery level (58%–89%) derived from deviceID so apps do not see
// a static 100% AC-plugged virtual battery.
func BatteryShellScript(deviceID string) string {
	h := sha256.Sum256([]byte("anyprofile-mobile-battery:" + deviceID))
	level := 58 + int(h[0])%32 // 58..89
	return fmt.Sprintf("dumpsys battery unplug; dumpsys battery set level %d; dumpsys battery set status 3", level)
}
