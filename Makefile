IMG ?= e2enetworks/cluster-api-provider-e2e:latest
CRD_OPTIONS ?= "crd"
KUBEBUILDER_ASSETS ?= $(shell go env GOPATH)/bin

CONTROLLER_GEN ?= $(GOBIN)/controller-gen
KUSTOMIZE ?= $(GOBIN)/kustomize
GOBIN ?= $(shell go env GOPATH)/bin

.PHONY: all
all: generate fmt vet build

##@ Development

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test: generate fmt vet
	go test ./... -coverprofile cover.out

.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

##@ Build

.PHONY: build
build: generate fmt vet
	go build -o bin/manager cmd/main.go

.PHONY: run
run: generate fmt vet
	go run cmd/main.go

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push:
	docker push $(IMG)

##@ Deployment

.PHONY: install
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found -f -

.PHONY: deploy
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found -f -

##@ Tools

.PHONY: controller-gen
controller-gen:
	@test -f $(CONTROLLER_GEN) || go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

.PHONY: kustomize
kustomize:
	@test -f $(KUSTOMIZE) || go install sigs.k8s.io/kustomize/kustomize/v5@latest

.PHONY: release
release: manifests kustomize
	mkdir -p out
	$(KUSTOMIZE) build config/default > out/infrastructure-components.yaml

.PHONY: clean
clean:
	rm -rf bin/ out/ cover.out
