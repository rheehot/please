go_library(
    name = "help",
    srcs = glob(
        ["*.go"],
        exclude = ["*_test.go"],
    ),
    visibility = ["PUBLIC"],
    deps = [
        "//rules",
        "//rules/bazel",
        "//src/cli",
        "//src/core",
        "//src/parse/asp",
        "//src/utils",
        "//third_party/go:go-flags",
        "//third_party/go:logging",
    ],
)

go_test(
    name = "help_test",
    srcs = ["help_test.go"],
    deps = [
        ":help",
        "//third_party/go:testify",
    ],
)

go_test(
    name = "rules_test",
    srcs = ["rules_test.go"],
    deps = [
        ":help",
        "//third_party/go:testify",
    ],
)
