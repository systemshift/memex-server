#!/bin/bash
set -e

echo "Building memex..."
go build -o memex cmd/memex/main.go

echo -e "\nTest 1: Initialize repository"
rm -f test.mx
./memex -repo test.mx add test/storage_test.go
if [ ! -f test.mx ]; then
    echo "FAIL: Repository file not created"
    exit 1
fi
echo "PASS: Repository initialized"

echo -e "\nTest 2: Add and retrieve file"
FIRST_ID=$(./memex -repo test.mx add test/integration_test.sh | grep -o 'ID: [a-f0-9]*' | cut -d' ' -f2)
if [ -z "$FIRST_ID" ]; then
    echo "FAIL: Could not get ID of added file"
    exit 1
fi
./memex -repo test.mx links $FIRST_ID
echo "PASS: File added and retrieved"

echo -e "\nTest 3: Add multiple files"
SECOND_ID=$(./memex -repo test.mx add test/storage_test.go | grep -o 'ID: [a-f0-9]*' | cut -d' ' -f2)
if [ -z "$SECOND_ID" ]; then
    echo "FAIL: Could not get ID of second file"
    exit 1
fi
./memex -repo test.mx links $SECOND_ID
echo "PASS: Multiple files handled"

echo -e "\nTest 4: Create link between files"
./memex -repo test.mx link $FIRST_ID $SECOND_ID "references" "Test link"
./memex -repo test.mx links $FIRST_ID | grep "references"
if [ $? -ne 0 ]; then
    echo "FAIL: Link not found"
    exit 1
fi
echo "PASS: Links created and retrieved"

echo -e "\nTest 5: Delete file"
./memex -repo test.mx delete $SECOND_ID
./memex -repo test.mx links $SECOND_ID 2>/dev/null
if [ $? -eq 0 ]; then
    echo "FAIL: Deleted file still accessible"
    exit 1
fi
echo "PASS: File deleted"

echo -e "\nAll tests passed!"
