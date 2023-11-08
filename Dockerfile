FROM ubuntu:jammy

RUN apt-get update && apt-get upgrade -y && apt-get install -y build-essential cmake git python3 python3-pip wget zip
RUN pip install conan
RUN conan profile detect

RUN printf '\n\
  include(default) \n\
  \n\
  [settings] \n\
  arch=wasm \n\
  os=Emscripten \n\
  \n\
  [tool_requires] \n\
  *: emsdk/3.1.44' > ~/.conan2/profiles/webassembly

RUN ls -la ~/.conan2/profiles
RUN cat ~/.conan2/profiles/webassembly

WORKDIR /opt

RUN git clone --depth 1 https://github.com/carimbolabs/carimbo.git

WORKDIR /opt/carimbo/build

RUN conan install ..  --output-folder=. --build=missing --profile=webassembly --settings compiler.cppstd=20 --settings build_type=Release

RUN cmake .. -DCMAKE_TOOLCHAIN_FILE=conan_toolchain.cmake -DCMAKE_BUILD_TYPE=Release

RUN cmake --build . --config Release

# FROM golang:1.21
# WORKDIR /opt
# COPY go.mod .
# COPY go.sum .
# RUN go mod download
# COPY . .
# RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o app

# FROM gcr.io/distroless/static-debian12
# COPY --from=0 /opt/app /
# CMD ["/app"]