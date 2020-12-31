#! /bin/bash
$GOEXEC build .
GOOS=windows $GOEXEC build .
GOOS=darwin $GOEXEC build -o stasi-blog-darwin .