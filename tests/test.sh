echo "--------------------------------------------------------------"
echo "                   Starting unit tests                        "
echo "--------------------------------------------------------------"

go clean -testcache && go test ./pkg/...

echo "--------------------------------------------------------------"
echo "                   Starting integration tests                 "
echo "--------------------------------------------------------------"

go clean -testcache && go test -v ./tests/testkit/...
