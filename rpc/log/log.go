// Package log
package log

import (
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetReportCaller(true)
}
