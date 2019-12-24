FROM hashicorp/terraform:light

RUN apk add -U make bash bash-completion vim

RUN wget https://github.com/flexkube/kubelet-rubber-stamp-helm-chart/archive/master.zip && \
    unzip master.zip && \
    mkdir -p /usr/src/libflexkube/charts && \
    mv kubelet-rubber-stamp-helm-chart-master /usr/src/libflexkube/charts/kubelet-rubber-stamp && \
    rm master.zip

RUN wget https://github.com/flexkube/kubernetes-helm-chart/archive/master.zip && \
    unzip master.zip && \
    mkdir -p /usr/src/libflexkube/charts && \
    mv kubernetes-helm-chart-master /usr/src/libflexkube/charts/kubernetes && \
    rm master.zip

RUN wget https://github.com/flexkube/kube-apiserver-helm-chart/archive/master.zip && \
    unzip master.zip && \
    mkdir -p /usr/src/libflexkube/charts && \
    mv kube-apiserver-helm-chart-master /usr/src/libflexkube/charts/kube-apiserver && \
    rm master.zip

RUN wget https://get.helm.sh/helm-v3.0.1-linux-amd64.tar.gz -O - | tar -xzvf - linux-amd64/helm && \
    mv linux-amd64/helm /usr/local/bin/

RUN wget -O /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/$(wget -q -O- https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl \
    && chmod +x /usr/local/bin/kubectl \
    && echo -e 'source /usr/share/bash-completion/bash_completion\nsource <(kubectl completion bash)' >> $HOME/.bashrc

RUN echo 'plugin_cache_dir = "/root/.terraform.d.host/plugin-cache"' >> /root/.terraformrc

RUN helm repo add stable https://kubernetes-charts.storage.googleapis.com

ENV KUBECONFIG=/root/libflexkube/e2e/kubeconfig