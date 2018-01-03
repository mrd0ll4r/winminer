# winminer

[![Build Status](https://api.travis-ci.org/mrd0ll4r/winminer.svg?branch=master)](https://travis-ci.org/mrd0ll4r/winminer)
[![Go Report Card](https://goreportcard.com/badge/github.com/mrd0ll4r/winminer)](https://goreportcard.com/report/github.com/mrd0ll4r/winminer)
[![GoDoc](https://godoc.org/github.com/mrd0ll4r/winminer?status.svg)](https://godoc.org/github.com/mrd0ll4r/winminer)
![Lines of Code](https://tokei.rs/b1/github/mrd0ll4r/winminer)

API wrapper for the winminer API

## Status

This is an ongoing effort to reverse-engineer the winminer API, particularly the live dashboard.

## Things to know

It seems like Winminer invalidates API tokens and websocket tokens after some time.
For this reason, it's recommended to update these tokens periodically.

## Stability

Is it stable? Probably not. Winminer doesn't guarantee anything for the API, so I don't guarantee anything for the wrapper :)

## License

MIT, see LICENSE.
