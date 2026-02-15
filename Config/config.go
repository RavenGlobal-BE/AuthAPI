package Config

/* This config file includes are changeable settings. They're gonna be hardcoded on compile-time. */

// Program configurations
var TestMode bool = false
var AllowVerbose bool = false

// Version & Build
var Version = "2.0"
var Build float32 = 3

// Server configurations
var Port int = 3000
