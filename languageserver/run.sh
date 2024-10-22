#!/bin/sh

SCRIPTPATH=$(dirname "$0")

if [ "$1" = "cadence" ] && [ "$2" = "language-server" ] ; then
	(cd "$SCRIPTPATH" && \
		go run -gcflags="all=-N -l" ./cmd/languageserver --enable-flow-client=false);
else
	flow "$@"
fi
