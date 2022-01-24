package main

import "github.com/fatih/color"

var ok = color.New(color.BgBlack, color.FgHiGreen, color.Bold)
var ok_title = color.New(color.BgBlack, color.FgHiGreen, color.Bold, color.ReverseVideo)

var print_ok = ok.FprintlnFunc()
var print_ok_title = ok_title.FprintlnFunc()
