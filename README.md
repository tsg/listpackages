# List OS packages

Golang PoC to list the installed packages on a particular system:

Currently supports:

* Debian and derivates - by reading and parsing the `/var/lib/dpkg/status` file
* RedHat and derivates - by exec'ing `rpm -qa`
