@echo off

del streamsink.exe

REM sqlite3 can only compile with these settings
set CGO_ENABLED=1
set CC=x86_64-w64-mingw32-gcc
set CXX=x86_64-w64-mingw32-g++

go build -o streamsink.exe