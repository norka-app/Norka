//go:build !windows

package main

import "github.com/norka-app/Norka/internal/loginstart"

func loginRegistry() loginstart.Registry { return nil }
