package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
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
}

func listRPMPackages() ([]Package, error) {
	format := "%{NAME}|%{VERSION}|%{RELEASE}|%{ARCH}|%{LICENSE}|%{INSTALLTIME}|%{SIZE}|%{SUMMARY}\\n"
	out, err := exec.Command("/usr/bin/rpm", "--qf", format, "-qa").Output()
	if err != nil {
		return nil, fmt.Errorf("Error running rpm -qa command: %v", err)
	}

	lines := strings.Split(string(out), "\n")
	packages := []Package{}
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		words := strings.SplitN(line, "|", 8)
		if len(words) < 8 {
			return nil, fmt.Errorf("Line '%s' doesn't have at least 7 elements", line)
		}
		pkg := Package{
			Name:    words[0],
			Version: words[1],
			Release: words[2],
			Arch:    words[3],
			License: words[4],
			// install time - 5
			// size - 6
			Summary: words[7],
		}
		ts, err := strconv.ParseInt(words[5], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error converting %s to string: %v", words[5], err)
		}
		pkg.InstallTime = time.Unix(ts, 0)

		pkg.Size, err = strconv.ParseUint(words[6], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error converting %s to string: %v", words[6], err)
		}

		packages = append(packages, pkg)

	}

	return packages, nil
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
		path := path.Join(cellarPath, packageDir.Name())
		versions, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("Error reading directory: %s: %v", path, err)
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
