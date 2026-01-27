package config

/* This config file includes are changeable settings. They're gonna be hardcoded on compile-time. */

// Program configurations
var Debug bool = false
var AllowVerbose bool = true

// Version & Build
var Version = "2.0"
var Build float32 = 1

// Server configurations
var Port int = 3001
