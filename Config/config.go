package Config

/* This config file includes are changeable settings. They're gonna be hardcoded on compile-time. */

// Program configurations
var TestMode bool = false
var AllowVerbose bool = true

// Version & Build
var Version = "2.0"
var Build float32 = 5

// Server configurations
var Port int = 3000
var AllowQuantumSecure bool = false

// Raven ONE configurations
var CdnEndpoint string = "https://cdn.raven.co.com/orgIcons/"
