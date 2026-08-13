package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstalledKernelsUsesAlpineModuleReleaseAndIgnoresBootFlavorAlias(t *testing.T) {
	root := t.TempDir()
	bootDir := filepath.Join(root, "boot")
	modulesDir := filepath.Join(root, "lib", "modules")
	if err := os.MkdirAll(filepath.Join(modulesDir, "6.12.103-0-virt"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bootDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bootDir, "vmlinuz-virt"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}

	got := getInstalledKernelsFromDirs(bootDir, modulesDir)
	want := []string{"6.12.103-0-virt"}
	if !reflect.DeepEqual(got.genericKernels, want) {
		t.Fatalf("generic kernels = %#v, want %#v", got.genericKernels, want)
	}
	if len(got.oemKernels) != 0 {
		t.Fatalf("unexpected OEM kernels: %#v", got.oemKernels)
	}

	comparison := compareKernelInventory("6.12.103-0-virt", got)
	if comparison.kernelNeedsReboot {
		t.Fatalf("matching Alpine kernel incorrectly needs reboot: %#v", comparison)
	}
	if comparison.latestInstalledKernel != "6.12.103-0-virt" {
		t.Fatalf("latest installed kernel = %q", comparison.latestInstalledKernel)
	}
}

func TestAlpineModuleReleaseDetectsPendingKernelReboot(t *testing.T) {
	inventory := installedKernelsStruct{genericKernels: []string{"6.12.104-0-virt"}}
	comparison := compareKernelInventory("6.12.103-0-virt", inventory)
	if !comparison.kernelNeedsReboot {
		t.Fatalf("newer installed Alpine kernel did not require reboot: %#v", comparison)
	}
	if comparison.latestInstalledKernel != "6.12.104-0-virt" {
		t.Fatalf("latest installed kernel = %q", comparison.latestInstalledKernel)
	}
}

func TestInstalledKernelSourcesAreDeduplicated(t *testing.T) {
	root := t.TempDir()
	bootDir := filepath.Join(root, "boot")
	modulesDir := filepath.Join(root, "modules")
	for _, dir := range []string{bootDir, filepath.Join(modulesDir, "6.8.0-1-generic")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bootDir, "vmlinuz-6.8.0-1-generic"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	got := getInstalledKernelsFromDirs(bootDir, modulesDir)
	if !reflect.DeepEqual(got.genericKernels, []string{"6.8.0-1-generic"}) {
		t.Fatalf("kernel sources were not deduplicated: %#v", got.genericKernels)
	}
}
