// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium

package main

import "github.com/klippa-app/go-pdfium/multi_threaded/worker"

func main() {
	worker.StartWorker(nil)
}
