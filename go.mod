module github.com/eyeskiller/fail2ban-notifier

go 1.26

toolchain go1.26.10

// Development and testing dependencies would go here if needed:
// github.com/stretchr/testify v1.8.4 // for enhanced testing
// github.com/golang/mock v1.6.0 // for mocking in tests

require (
	github.com/maxmind/mmdbwriter v1.2.0
	github.com/oschwald/maxminddb-golang/v2 v2.1.1
	golang.org/x/term v0.44.0
)

require (
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba // indirect
	golang.org/x/sys v0.46.0 // indirect
)
