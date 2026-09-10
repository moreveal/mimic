#!/bin/bash -eu

export CXX="${CXX} -lresolv" # required by Go 1.20

compile_go_fuzzer github.com/bogdanfinn/quic-go-utls/fuzzing/frames Fuzz frame_fuzzer
compile_go_fuzzer github.com/bogdanfinn/quic-go-utls/fuzzing/header Fuzz header_fuzzer
compile_go_fuzzer github.com/bogdanfinn/quic-go-utls/fuzzing/transportparameters Fuzz transportparameter_fuzzer
compile_go_fuzzer github.com/bogdanfinn/quic-go-utls/fuzzing/tokens Fuzz token_fuzzer
compile_go_fuzzer github.com/bogdanfinn/quic-go-utls/fuzzing/handshake Fuzz handshake_fuzzer
