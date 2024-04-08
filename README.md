# gtdd

gtdd is a tool to parallelize the execution of End-to-End test suites.
It tries to automatically discover the dependencies between tests into
the input test suite to generate some parallel schedules that respect
the dependencies discovered.


----

## To start using gtdd

Ensure you have a working [Go environment], and execute the following
commands.

```bash
git clone https://github.com/pako-23/gtdd.git
cd gtdd
make
```

[Go environment]: https://go.dev/doc/install
