echo "--------------------------------------------------------------"
echo "                   Starting Unit Tests                        "
echo "--------------------------------------------------------------"

go clean -testcache && go test ./pkg/...

echo "--------------------------------------------------------------"
echo "                   Starting Integration Tests                 "
echo "--------------------------------------------------------------"

go clean -testcache && go test -v ./tests/testkit/...
