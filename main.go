package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	sysinfo "github.com/elastic/go-sysinfo"
)

type Package struct {
	Name        string
	Version     string
	Release     string
	Arch        string
	License     string
	InstallTime time.Time
	Size        uint64
	Summary     string
	URL         string
}

func listDebPackages() ([]Package, error) {
	statusFile := "/var/lib/dpkg/status"
	file, err := os.Open(statusFile)
	if err != nil {
		return nil, fmt.Errorf("Error opening '%s': %v", statusFile, err)
	}
	defer file.Close()

	packages := []Package{}
	pkg := &Package{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(strings.TrimSpace(line)) == 0 {
			// empty line signals new package
			packages = append(packages, *pkg)
			pkg = &Package{}
			continue
		}
		if strings.HasPrefix(line, " ") {
			// not interested in multi-lines for now
			continue
		}
		words := strings.SplitN(line, ":", 2)
		if len(words) != 2 {
			return nil, fmt.Errorf("The following line was unexpected (no ':' found): '%s'", line)
		}
		value := strings.TrimSpace(words[1])
		switch strings.ToLower(words[0]) {
		case "package":
			pkg.Name = value
		case "architecture":
			pkg.Arch = value
		case "version":
			pkg.Version = value
		case "description":
			pkg.Summary = value
		case "installed-size":
			pkg.Size, err = strconv.ParseUint(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Error converting %s to int: %v", value, err)
			}
		default:
			continue
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error scanning file: %v", err)
	}
	return packages, nil
}

func listBrewPackages() ([]Package, error) {
	cellarPath := "/usr/local/Cellar"

	cellarInfo, err := os.Stat(cellarPath)
	if err != nil {
		return nil, fmt.Errorf("Homebrew cellar not found in %s: %v", cellarPath, err)
	}
	if !cellarInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", cellarPath)
	}

	packageDirs, err := ioutil.ReadDir(cellarPath)
	if err != nil {
		return nil, fmt.Errorf("Error reading directory %s: %v", cellarPath, err)
	}

	packages := []Package{}
	for _, packageDir := range packageDirs {
		if !packageDir.IsDir() {
			continue
		}
		pkgPath := path.Join(cellarPath, packageDir.Name())
		versions, err := ioutil.ReadDir(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("Error reading directory: %s: %v", pkgPath, err)
		}
		for _, version := range versions {
			if !version.IsDir() {
				continue
			}
			pkg := Package{
				Name:        packageDir.Name(),
				Version:     version.Name(),
				InstallTime: version.ModTime(),
			}

			// read formula
			formulaPath := path.Join(cellarPath, pkg.Name, pkg.Version, ".brew", pkg.Name+".rb")
			file, err := os.Open(formulaPath)
			if err != nil {
				//fmt.Printf("WARNING: Can't get formula for package %s-%s\n", pkg.Name, pkg.Version)
				// TODO: follow the path from INSTALL_RECEIPT.json to find the formula
				continue
			}
			scanner := bufio.NewScanner(file)
			count := 15 // only look into the first few lines of the formula
			for scanner.Scan() {
				count -= 1
				if count == 0 {
					break
				}
				line := scanner.Text()
				if strings.HasPrefix(line, "  desc ") {
					pkg.Summary = strings.Trim(line[7:], " \"")
				} else if strings.HasPrefix(line, "  homepage ") {
					pkg.URL = strings.Trim(line[11:], " \"")
				}
			}

			packages = append(packages, pkg)
		}
	}
	return packages, nil
}

func main() {

	host, err := sysinfo.Host()
	if err != nil {
		fmt.Println("Error getting the OS: %v", err)
		os.Exit(1)
	}

	hostInfo := host.Info()
	if hostInfo.OS == nil {
		fmt.Println("No OS info from sysinfo.Host")
		os.Exit(1)
	}

	var packages []Package
	switch hostInfo.OS.Family {
	case "redhat":
		packages, err = listRPMPackages()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "debian":
		packages, err = listDebPackages()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "darwin":
		packages, err = listBrewPackages()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Println("I don't know how to get pacakges on OS family %s", hostInfo.OS.Family)
		os.Exit(1)
		return
	}

	for _, pkg := range packages {
		bt, _ := json.Marshal(pkg)
		fmt.Println(string(bt))
	}
}
