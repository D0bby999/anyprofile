// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
)

// HardwareTemplate describes one Avo device profile JSON from profiles/devices.
type HardwareTemplate struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
	Display    struct {
		Width  int `json:"width"`
		Height int `json:"height"`
		DPI    int `json:"dpi"`
	} `json:"display"`
	Hardware *struct {
		SoC *struct {
			Manufacturer string            `json:"manufacturer"`
			Model        string            `json:"model"`
			Platform     string            `json:"platform"`
			Board        string            `json:"board"`
			Hardware     string            `json:"hardware"`
			Properties   map[string]string `json:"properties"`
		} `json:"soc"`
		CPU *struct {
			Clusters []cpuCluster `json:"clusters"`
		} `json:"cpu"`
		GPU *struct {
			GLVendor        string  `json:"glVendor"`
			GLRenderer      string  `json:"glRenderer"`
			GLVersion       string  `json:"glVersion"`
			VKName          string  `json:"vkName"`
			VKVendorID      *uint32 `json:"vkVendorID"`
			VKDeviceID      *uint32 `json:"vkDeviceID"`
			VKDriverVersion *uint32 `json:"vkDriverVersion"`
			VKDriverID      *uint32 `json:"vkDriverID"`
			VKDriverName    string  `json:"vkDriverName"`
			VKDriverInfo    string  `json:"vkDriverInfo"`
		} `json:"gpu"`
	} `json:"hardware"`
}

type cpuCluster struct {
	Cores       int  `json:"cores"`
	Implementer int  `json:"implementer"`
	Part        int  `json:"part"`
	Variant     *int `json:"variant"`
	Revision    *int `json:"revision"`
	MaxFreqKHz  *int `json:"maxFreqKHz"`
	MinFreqKHz  *int `json:"minFreqKHz"`
}

var legacyTemplateAlias = map[string]string{
	"pixel8":        "google-pixel-8-pro",
	"pixel9":        "google-pixel-9-pro",
	"s24ultra":      "samsung-galaxy-s24-ultra",
	"s24":           "samsung-galaxy-s24",
	"s23ultra":      "samsung-galaxy-s24-ultra",
	"a54":           "samsung-galaxy-a54",
	"xiaomi14pro":   "xiaomi-14",
	"xiaomi13ultra": "xiaomi-14",
	"oneplus12":     "oneplus-12",
	"oppofindx6pro": "oppo-reno8-5g",
}

// LaunchSpec contains the identity property file content plus optional CPU/SoC flags.
type LaunchSpec struct {
	PropertiesText string
	Hardware       string
	CPUClusters    string
}

// ListTemplates returns available Avo device hardware profiles sorted by name.
func ListTemplates(profilesDir string) []HardwareTemplate {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil
	}
	var out []HardwareTemplate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if t, ok := loadTemplateFile(filepath.Join(profilesDir, e.Name())); ok {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func loadTemplate(profilesDir, name string) (HardwareTemplate, bool) {
	if profilesDir == "" || name == "" {
		return HardwareTemplate{}, false
	}
	if alias, ok := legacyTemplateAlias[strings.ToLower(name)]; ok {
		name = alias
	}
	clean := filepath.Base(name)
	if !strings.HasSuffix(clean, ".json") {
		clean += ".json"
	}
	return loadTemplateFile(filepath.Join(profilesDir, clean))
}

func loadTemplateFile(path string) (HardwareTemplate, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return HardwareTemplate{}, false
	}
	var t HardwareTemplate
	if json.Unmarshal(b, &t) != nil || t.ID == "" {
		return HardwareTemplate{}, false
	}
	return t, true
}

// BuildLaunchSpec converts AnyProfile's Device + optional hardware template
// JSON into Cuttlefish early-init properties and avo-vmm --hardware / --cpu-clusters flags.
// enableDebugLayers must be true only once the instance's /data/local/debug/{vulkan,gles}
// contains the layer .so files, otherwise ANGLE fails vkCreateInstance (-6).
func BuildLaunchSpec(rt Runtime, d mobile.Device, enableDebugLayers bool) LaunchSpec {
	props := make(map[string]string)
	setProp := func(k, v string) {
		if k != "" && vm.ValidPropValue(v) {
			props[k] = v
		}
	}

	var spec LaunchSpec
	id, hasID := vm.ParseIdentity(d.IdentityJSON)
	if hasID {
		setProp("ro.product.model", id.Model)
		setProp("ro.product.brand", id.Brand)
		setProp("ro.product.manufacturer", id.Manufacturer)
		setProp("ro.product.device", id.Device)
		setProp("ro.product.name", id.Product)
		setProp("ro.product.board", id.Board)
		setProp("ro.build.fingerprint", id.Fingerprint)
		setProp("ro.build.id", id.BuildID)
		setProp("ro.build.version.incremental", id.Incremental)
		setProp("ro.serialno", id.Serial)
		setProp("ro.boot.serialno", id.Serial)
		// Keep partition build fingerprints and security_patch intact so KeyMint
		// unwraps FBE keys created during the shared GMS base image setup.
		for _, pfx := range []string{"vendor.", "odm.", "product.", "system.", "system_ext."} {
			setProp("ro.product."+pfx+"model", id.Model)
			setProp("ro.product."+pfx+"brand", id.Brand)
			setProp("ro.product."+pfx+"manufacturer", id.Manufacturer)
			setProp("ro.product."+pfx+"device", id.Device)
			setProp("ro.product."+pfx+"name", id.Product)
		}
		if validHardwareName(id.Hardware) {
			spec.Hardware = id.Hardware
		}
	}

	tplName := id.Template
	if t, ok := loadTemplate(rt.ProfilesDir, tplName); ok {
		for k, v := range t.Properties {
			setProp(k, v)
		}
		if t.Hardware != nil && t.Hardware.SoC != nil {
			for k, v := range t.Hardware.SoC.Properties {
				setProp(k, v)
			}
			hw := t.Hardware.SoC.Hardware
			if hw == "" {
				if t.Hardware.SoC.Manufacturer == "QTI" {
					hw = "qcom"
				} else {
					hw = t.Hardware.SoC.Platform
				}
			}
			if validHardwareName(hw) {
				spec.Hardware = hw
			}
		}
		if t.Hardware != nil && t.Hardware.CPU != nil {
			if c := engineClusters(t.Hardware.CPU.Clusters, d.CPUs); c != "" {
				spec.CPUClusters = c
			}
		}
		if t.Hardware != nil && t.Hardware.GPU != nil {
			g := t.Hardware.GPU
			setProp("debug.avo.gl.vendor", g.GLVendor)
			setProp("debug.avo.gl.renderer", g.GLRenderer)
			setProp("debug.avo.gl.version", g.GLVersion)
			setProp("debug.avo.vk.name", g.VKName)
			setProp("debug.avo.vk.driver_name", g.VKDriverName)
			setProp("debug.avo.vk.driver_info", g.VKDriverInfo)
			if g.VKVendorID != nil {
				setProp("debug.avo.vk.vendor", fmt.Sprintf("0x%x", *g.VKVendorID))
			}
			if g.VKDeviceID != nil {
				setProp("debug.avo.vk.device", fmt.Sprintf("0x%x", *g.VKDeviceID))
			}
			if g.VKDriverVersion != nil {
				setProp("debug.avo.vk.driver", fmt.Sprintf("0x%x", *g.VKDriverVersion))
			}
			if g.VKDriverID != nil {
				setProp("debug.avo.vk.driver_id", fmt.Sprintf("%d", *g.VKDriverID))
			}
		}
	}
	if hasID && props["debug.avo.gl.vendor"] == "" && id.GLVendor != "" {
		setProp("debug.avo.gl.vendor", id.GLVendor)
		setProp("debug.avo.gl.renderer", id.GLRenderer)
	}
	if enableDebugLayers && rt.LayersDir != "" {
		if exists(filepath.Join(rt.LayersDir, "libVkLayer_avo_gfxstream_sync.so")) {
			setProp("debug.vulkan.layer.9", "VK_LAYER_AVO_gfxstream_sync")
			// Cuttlefish Android 16 ANGLE supports exposeES32ForTesting; unlock OpenGL ES 3.2 (0x30002).
			setProp("debug.avo.gl.es32", "1")
			setProp("ro.opengles.version", "196610")
		}
		if props["debug.avo.gl.renderer"] != "" && exists(filepath.Join(rt.LayersDir, "libGLES_avo_profile.so")) {
			setProp("debug.gles.layers", "libGLES_avo_profile.so")
		}
		if (props["debug.avo.vk.name"] != "" || props["debug.avo.gl.renderer"] != "") && exists(filepath.Join(rt.LayersDir, "libVkLayer_avo_profile.so")) {
			setProp("debug.vulkan.layers", "VK_LAYER_AVO_profile")
		}
	}

	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(props[k])
		b.WriteByte('\n')
	}
	spec.PropertiesText = b.String()
	return spec
}

func validHardwareName(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
		if !ok {
			return false
		}
	}
	return true
}

func (c cpuCluster) presentedMIDR() uint32 {
	var v, r uint32
	if c.Variant != nil {
		v = uint32(*c.Variant & 0xf)
	}
	if c.Revision != nil {
		r = uint32(*c.Revision & 0xf)
	}
	midr := uint32(c.Implementer&0xff)<<24 | v<<20 | 0xf<<16 | uint32(c.Part&0xfff)<<4 | r
	// Avoid Cortex-A53 erratum traps (<r0p3) by presenting r0p4.
	if c.Implementer == 0x41 && c.Part == 0xD03 && v == 0 && r < 3 {
		return (midr &^ 0xf) | 4
	}
	return midr
}

func (c cpuCluster) trapsTimer() bool {
	return c.Implementer == 0x41 && c.Part == 0xD09 // Cortex-A73 vDSO trap
}

func engineClusters(clusters []cpuCluster, vcpus int) string {
	if vcpus <= 0 || len(clusters) == 0 {
		return ""
	}
	ordered := append([]cpuCluster(nil), clusters...)
	sort.SliceStable(ordered, func(i, j int) bool {
		fi, fj := 0, 0
		if ordered[i].MaxFreqKHz != nil {
			fi = *ordered[i].MaxFreqKHz
		}
		if ordered[j].MaxFreqKHz != nil {
			fj = *ordered[j].MaxFreqKHz
		}
		return fi < fj
	})
	left := vcpus
	var kept []cpuCluster
	for i := len(ordered) - 1; i >= 0 && left > 0; i-- {
		cl := ordered[i]
		if cl.Cores <= 0 {
			continue
		}
		if cl.Cores > left {
			cl.Cores = left
		}
		left -= cl.Cores
		kept = append([]cpuCluster{cl}, kept...)
	}
	if len(kept) == 0 {
		return ""
	}
	if left > 0 {
		kept[0].Cores += left
	}
	parts := make([]string, 0, len(kept))
	for _, cl := range kept {
		if cl.MaxFreqKHz == nil || *cl.MaxFreqKHz <= 1 || cl.trapsTimer() {
			return ""
		}
		top := *cl.MaxFreqKHz
		low := 300000
		if low > top/2 {
			low = top / 2
		}
		if cl.MinFreqKHz != nil && *cl.MinFreqKHz > 0 && *cl.MinFreqKHz < top {
			low = *cl.MinFreqKHz
		}
		parts = append(parts, fmt.Sprintf("%dx%08x@%d-%d", cl.Cores, cl.presentedMIDR(), low, top))
	}
	return strings.Join(parts, ",")
}
