echo "-------------------------------"
echo "    Starting unit tests        "
echo "-------------------------------"

go test ./pkg/...

echo "-------------------------------"
echo "  Starting integration tests   "
echo "-------------------------------"

go test ./tests/...
