package cmd

import (
	"github.com/mattsolo1/grove-core/logging"
)

var (
	log  = logging.NewLogger("grove-context")
	ulog = logging.NewUnifiedLogger("grove-context")
)