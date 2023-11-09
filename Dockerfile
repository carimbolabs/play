FROM golang:1.21
WORKDIR /opt
COPY go.mod .
#COPY go.sum .
#RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o app

FROM debian:bookworm-slim

RUN <<EOF
set -eux
apt-get update
apt-get install -y build-essential cmake git
EOF

COPY --from=node:20-bookworm /usr/local/bin/node /usr/local/bin/node
COPY --from=node:20-bookworm /usr/local/include/node /usr/local/include/node
COPY --from=node:20-bookworm /usr/local/lib/node_modules /usr/local/lib/node_modules

COPY --from=python:3.11-bookworm /usr/local/bin/python3 /usr/local/bin/python3
COPY --from=python:3.11-bookworm /usr/local/lib/python3.11 /usr/local/lib/python3.11
COPY --from=python:3.11-bookworm /usr/local/lib/libpython3.11.so.1.0 /usr/local/lib/libpython3.11.so.1.0

RUN <<-EOF
set -eux
python3 -m pip install conan
conan profile detect

cat <<PROFILE > ~/.conan2/profiles/webassembly
include(default)

[settings]
arch=wasm
os=Emscripten

[tool_requires]
*: emsdk/3.1.44
PROFILE
EOF

WORKDIR /opt
RUN git clone --depth 1 https://github.com/carimbolabs/carimbo.git
WORKDIR /opt/carimbo/build
RUN <<EOF
set -eux
conan install ..  --output-folder=. --build=missing --profile=webassembly --settings compiler.cppstd=20 --settings build_type=Release
cmake .. -DCMAKE_TOOLCHAIN_FILE=conan_toolchain.cmake -DCMAKE_BUILD_TYPE=Release
EOF
