#!/usr/bin/env bash
#
# Generate all protobuf bindings.
# Run from repository root.
set -e
set -u

PROTOC_VERSION=${PROTOC_VERSION:-3.20.1}
PROTOC_BIN=${PROTOC_BIN:-protoc}
GOIMPORTS_BIN=${GOIMPORTS_BIN:-goimports}
PROTOC_GEN_GOGOFAST_BIN=${PROTOC_GEN_GOGOFAST_BIN:-protoc-gen-gogofast}
PROTOC_GEN_GO_BIN=${PROTOC_GEN_GO_BIN:-protoc-gen-go}
PROTOC_GEN_GO_GRPC_BIN=${PROTOC_GEN_GO_GRPC_BIN:-protoc-gen-go-grpc}
PROTOC_GEN_GO_VTPROTO_BIN=${PROTOC_GEN_GO_VTPROTO_BIN:-protoc-gen-go-vtproto}
PROTOC_GEN_GOTAG_BIN=${PROTOC_GEN_GOTAG_BIN:-protoc-gen-gotag}

if ! [[ "scripts/genproto.sh" =~ $0 ]]; then
  echo "must be run from repository root"
  exit 255
fi

if ! [[ $(${PROTOC_BIN} --version) == *"${PROTOC_VERSION}"* ]]; then
  echo "could not find protoc ${PROTOC_VERSION}, is it installed + in PATH?"
  exit 255
fi

mkdir -p /tmp/protobin/
cp ${PROTOC_GEN_GOGOFAST_BIN} /tmp/protobin/protoc-gen-gogofast
cp ${PROTOC_GEN_GO_BIN} /tmp/protobin/protoc-gen-go
cp ${PROTOC_GEN_GO_GRPC_BIN} /tmp/protobin/protoc-gen-go-grpc
cp ${PROTOC_GEN_GO_VTPROTO_BIN} /tmp/protobin/protoc-gen-go-vtproto
cp ${PROTOC_GEN_GOTAG_BIN} /tmp/protobin/protoc-gen-gotag
echo ">> building protoc-gen-go-grpc-vtpool from source"
go build -o /tmp/protobin/protoc-gen-go-grpc-vtpool ./pkg/vtproto/gen/
PATH=${PATH}:/tmp/protobin
VTPOOL_PROTO_DIR="$(pwd)/pkg/vtproto/gen"
GOGOPROTO_ROOT="$(GO111MODULE=on go list -modfile=.bingo/protoc-gen-gogofast.mod -f '{{ .Dir }}' -m github.com/gogo/protobuf)"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"
GOOGLE_PROTO_PATH="${GOGOPROTO_ROOT}/protobuf"
VTPROTO_ROOT="$(GO111MODULE=on go list -modfile=.bingo/protoc-gen-go-vtproto.mod -f '{{ .Dir }}' -m github.com/planetscale/vtprotobuf)"
VTPROTO_INCLUDE="${VTPROTO_ROOT}/include"
GOTAG_ROOT="$(GO111MODULE=on go list -modfile=.bingo/protoc-gen-gotag.mod -f '{{ .Dir }}' -m github.com/srikrsna/protoc-gen-gotag)"

# M-overrides for well-known types whose go_package in the gogo bundle
# is not a valid full import path (protoc-gen-go requires one).
WKT_GO_OPT="--go_opt=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb"
WKT_GO_OPT="${WKT_GO_OPT} --go_opt=Mgoogle/protobuf/duration.proto=google.golang.org/protobuf/types/known/durationpb"
WKT_GO_OPT="${WKT_GO_OPT} --go_opt=Mgoogle/protobuf/timestamp.proto=google.golang.org/protobuf/types/known/timestamppb"
WKT_GO_OPT="${WKT_GO_OPT} --go_opt=Mgoogle/protobuf/wrappers.proto=google.golang.org/protobuf/types/known/wrapperspb"
WKT_GO_OPT="${WKT_GO_OPT} --go_opt=Mgoogle/protobuf/struct.proto=google.golang.org/protobuf/types/known/structpb"
WKT_GO_OPT="${WKT_GO_OPT} --go_opt=Mgoogle/protobuf/empty.proto=google.golang.org/protobuf/types/known/emptypb"
WKT_GO_OPT="${WKT_GO_OPT} --go_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb"
WKT_VTPROTO_OPT="--go-vtproto_opt=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb"
WKT_VTPROTO_OPT="${WKT_VTPROTO_OPT} --go-vtproto_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb"
WKT_VTPROTO_OPT="${WKT_VTPROTO_OPT} --go-vtproto_opt=Mgoogle/protobuf/duration.proto=google.golang.org/protobuf/types/known/durationpb"
WKT_VTPROTO_OPT="${WKT_VTPROTO_OPT} --go-vtproto_opt=Mgoogle/protobuf/timestamp.proto=google.golang.org/protobuf/types/known/timestamppb"
WKT_GRPC_OPT="--go-grpc_opt=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb"
WKT_GRPC_OPT="${WKT_GRPC_OPT} --go-grpc_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb"
WKT_GRPC_OPT="${WKT_GRPC_OPT} --go-grpc_opt=Mgoogle/protobuf/duration.proto=google.golang.org/protobuf/types/known/durationpb"
WKT_GRPC_OPT="${WKT_GRPC_OPT} --go-grpc_opt=Mgoogle/protobuf/timestamp.proto=google.golang.org/protobuf/types/known/timestamppb"
WKT_VTPOOL_OPT="--go-grpc-vtpool_opt=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb"
WKT_VTPOOL_OPT="${WKT_VTPOOL_OPT} --go-grpc-vtpool_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb"
WKT_VTPOOL_OPT="${WKT_VTPOOL_OPT} --go-grpc-vtpool_opt=Mgoogle/protobuf/duration.proto=google.golang.org/protobuf/types/known/durationpb"
WKT_VTPOOL_OPT="${WKT_VTPOOL_OPT} --go-grpc-vtpool_opt=Mgoogle/protobuf/timestamp.proto=google.golang.org/protobuf/types/known/timestamppb"

echo "generating code"
pushd "pkg"

# ---------------------------------------------------------------------------
# Directories migrated to protoc-gen-go + vtprotobuf.
# ---------------------------------------------------------------------------
VT_DIRS="store/labelpb store/storepb/prompb store/storepb info/infopb exemplars/exemplarspb rules/rulespb targets/targetspb store/hintspb queryfrontend metadata/metadatapb api/query/querypb"
GRPC_DIRS="store/storepb info/infopb exemplars/exemplarspb rules/rulespb targets/targetspb metadata/metadatapb api/query/querypb"

# Directories that get gotag post-processing to rewrite Go json struct tags
# to lowerCamelCase (without omitempty).
GOTAG_DIRS="${VT_DIRS}"

VT_FEATURES="marshal+unmarshal+size+pool+equal+clone"

echo "generating vtproto code"
for dir in ${VT_DIRS}; do
  echo "  ${dir}"

  GRPC_FLAGS=""
  VTPOOL_FLAGS=""
  if echo " ${GRPC_DIRS} " | grep -q " ${dir} "; then
    GRPC_FLAGS="--go-grpc_out=. --go-grpc_opt=paths=source_relative ${WKT_GRPC_OPT}"
    VTPOOL_FLAGS="--go-grpc-vtpool_out=. --go-grpc-vtpool_opt=paths=source_relative ${WKT_VTPOOL_OPT}"
  fi

  ${PROTOC_BIN} \
    --go_out=. --go_opt=paths=source_relative ${WKT_GO_OPT} \
    ${GRPC_FLAGS} \
    ${VTPOOL_FLAGS} \
    --go-vtproto_out=. \
    --go-vtproto_opt=paths=source_relative,features=${VT_FEATURES} \
    --go-vtproto_opt=ignoreUnknownFields=** \
    ${WKT_VTPROTO_OPT} \
    -I=. \
    -I="${GOOGLE_PROTO_PATH}" \
    -I="${VTPROTO_INCLUDE}" \
    -I="${GOTAG_ROOT}" \
    -I="${VTPOOL_PROTO_DIR}" \
    ${dir}/*.proto

  pushd ${dir}
  ${GOIMPORTS_BIN} -w *.pb.go
  popd
done

# This is a requirement to get the most performance out of string interning.
# If we use the stdlib unique package, string interning tends to deadlock
# and increase latency considerably.
echo "replacing stdlib unique with pkg/unique in vtproto generated code"
for dir in ${VT_DIRS}; do
  for f in ${dir}/*_vtproto.pb.go; do
    [ -f "$f" ] || continue
    if grep -q 'unique "unique"' "$f"; then
      echo "  ${f}"
      sed -i.bak \
        -e 's|unique "unique"|unique "github.com/thanos-io/thanos/pkg/unique"|' \
        -e 's|unique\.Make\[string\](\(.*\))\.Value()|unique.Make(\1).Value()|g' \
        "$f"
      rm -f "${f}.bak"
    fi
  done
done

# Re-run goimports to clean up any unused imports after the unique rewrite.
for dir in ${VT_DIRS}; do
  pushd ${dir}
  ${GOIMPORTS_BIN} -w *_vtproto.pb.go 2>/dev/null || true
  popd
done

echo "rewriting json struct tags (gotag)"
for dir in ${GOTAG_DIRS}; do
  echo "  ${dir}"
  ${PROTOC_BIN} \
    --gotag_out=:. \
    --gotag_opt=paths=source_relative \
    -I=. \
    -I="${GOOGLE_PROTO_PATH}" \
    -I="${VTPROTO_INCLUDE}" \
    -I="${GOTAG_ROOT}" \
    -I="${VTPOOL_PROTO_DIR}" \
    ${dir}/*.proto
done

popd

# Generate vendored Cortex protobufs.
CORTEX_DIRS="cortex/querier/queryrange/ cortex/querier/stats"
pushd "internal"
for dir in ${CORTEX_DIRS}; do
  ${PROTOC_BIN} --gogofast_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,plugins=grpc:. \
    -I=../pkg \
    -I="${GOGOPROTO_PATH}" \
    -I="${VTPROTO_INCLUDE}" \
    -I="${GOTAG_ROOT}" \
    -I=. \
    ${dir}/*.proto

  pushd ${dir}
  sed -i.bak -E 's/import _ \"gogoproto\"//g' *.pb.go
  sed -i.bak -E 's/_ \"google\/protobuf\"//g' *.pb.go
  sed -i.bak -E 's/\"cortex\/cortexpb\"/\"github.com\/thanos-io\/thanos\/internal\/cortex\/cortexpb\"/g' *.pb.go
  rm -f *.bak
  ${GOIMPORTS_BIN} -w *.pb.go
  popd
done
popd
