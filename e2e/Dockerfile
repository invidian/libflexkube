FROM golang:1.23-alpine3.20

RUN apk add -U make bash bash-completion vim coreutils

RUN wget https://get.helm.sh/helm-v3.15.1-linux-amd64.tar.gz -O- | tar -xzvf - -C /usr/local/bin --strip-components=1 linux-amd64/helm

RUN wget -O /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/$(wget -q -O- https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    echo -e 'source /usr/share/bash-completion/bash_completion\nsource <(kubectl completion bash)' >> $HOME/.bashrc

RUN VERSION=0.57.1 OS=linux && \
    wget "https://github.com/vmware-tanzu/sonobuoy/releases/download/v${VERSION}/sonobuoy_${VERSION}_${OS}_amd64.tar.gz" -O- | \
    tar -xzf - -C /usr/local/bin

RUN helm repo add flexkube https://flexkube.github.io/charts/

ENV KUBECONFIG=/root/libflexkube/e2e/kubeconfig
