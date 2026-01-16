package cmd

import (
	"github.com/grovetools/core/logging"
)

var (
	log  = logging.NewLogger("grove-context")
	ulog = logging.NewUnifiedLogger("grove-context")
)