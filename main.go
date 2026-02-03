package main

import (
	"errors"
	"os"
	"runtime"

	"github.com/cnrancher/hangar/pkg/commands"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/sirupsen/logrus"
)

func main() {
	setup()
	if err := commands.Execute(os.Args[1:]); err != nil {
		cleanup()
		if errors.Is(err, commands.ErrAborted) {
			os.Exit(130)
		}
		logrus.Fatal(err)
	}
	cleanup()
}

func cleanup() {
	if err := os.RemoveAll(utils.HangarCacheDir()); err != nil {
		logrus.Warnf("failed to delete %q: %v", utils.HangarCacheDir(), err)
	}
	logrus.Debugf("cleanup %q", utils.HangarCacheDir())
}

func setup() {
	if runtime.GOOS == "windows" {
		logrus.Panicf("unsupported OS: %v", runtime.GOOS)
	}
}
