ifndef UNAME_S
UNAME_S := $(shell uname -s)
endif

ifndef UNAME_P
UNAME_P := $(shell uname -p)
endif

ifndef UNAME_M
UNAME_M := $(shell uname -m)
endif

GGML_METAL_PATH_RESOURCES := $(abspath ./whisper.cpp)
BUILD_DIR := build
MODELS_DIR := models
EXAMPLES_DIR := $(wildcard examples/*)
INCLUDE_PATH := $(abspath ./whisper.cpp/include):$(abspath ./whisper.cpp/ggml/include)
LIBRARY_PATH := $(abspath ./whisper.cpp):$(abspath ./whisper.cpp/build/src):$(abspath ./whisper.cpp/build/ggml/src):$(abspath ./whisper.cpp/build/ggml/src/ggml-metal):$(abspath ./whisper.cpp/build/ggml/src/ggml-blas)

ifeq ($(GGML_CUDA),1)
	LIBRARY_PATH := $(LIBRARY_PATH):$(CUDA_PATH)/targets/$(UNAME_M)-linux/lib/
	BUILD_FLAGS := -ldflags "-extldflags '-lcudart -lcuda -lcublas'"
endif

ifeq ($(UNAME_S),Darwin)
	EXT_LDFLAGS :=-lggml -lggml-cpu -lggml-metal -lggml-blas -lggml-base
endif

whisper:
	@echo Build whisper
	@${MAKE} -C ./whisper.cpp libwhisper.a

build: whisper

	@go mod tidy
	@echo Build
ifeq ($(UNAME_S),Darwin)
	@C_INCLUDE_PATH=${INCLUDE_PATH} LIBRARY_PATH=${LIBRARY_PATH} GGML_METAL_PATH_RESOURCES=${GGML_METAL_PATH_RESOURCES} go build ${BUILD_FLAGS} -ldflags "-extldflags '$(EXT_LDFLAGS)'"
else
	@C_INCLUDE_PATH=${INCLUDE_PATH} LIBRARY_PATH=${LIBRARY_PATH} go build ${BUILD_FLAGS} -o ${BUILD_DIR}/$(notdir $@) ./$@
endif

