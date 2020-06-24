# Streaming Globber for Go

This repository contains a package that provides functionality very similar to
filepath.Glob, but avoids the following pitfalls (at the expense of complexity):

 - filepath.Glob cannot be cancelled. If it starts running on an extremely large
   directory tree, your goroutine is blocked until it finishes.

 - filepath.Glob collects each directory's entire contents in a slice and sorts
   it - this is expensive for large directories.

*This is not an officially supported Google product*.

Aside from avoiding those pitfalls (and, by necessity, dropping the un-spoken
promise that filepath.Glob returns directory contents sorted), this library
strives to produce the same results as filepath.Glob. In fact, if when reading
the code you think "this is an awkward retrofit of asynchronous API on to a
synchronous algorithm", your observation is correct. This code was copy-pasted
from the Go standard library and modified to provide asynchronous operation
without altering the fundamental globbing algorithm: confidence in the
algorithm's equivalence to filepath.Glob is valued before the code's independent
beauty.
