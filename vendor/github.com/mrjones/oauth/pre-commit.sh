#!/bin/bash
# ln -s $PWD/pre-commit.sh .git/hooks/pre-commit
go test *.go
RESULT=$?
if [[ $RESULT != 0 ]]; then
    echo "REJECTING COMMIT (test failed with status: $RESULT)"
    exit 1;
fi

go fmt *.go
for e in $(ls examples); do 
    go build examples/$e/*.go
    RESULT=$?
    if [[ $RESULT != 0 ]]; then
        echo "REJECTING COMMIT (Examples failed to compile)"
        exit $RESULT;
    fi
    go fmt examples/$e/*.go
done

exit 0
