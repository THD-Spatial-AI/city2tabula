FROM debian:bookworm-slim

ARG DEBIAN_FRONTEND=noninteractive
ENV GO_VERSION=1.23.3
ENV JAVA_VERSION=25

# -----------------------------
# Install system dependencies
# -----------------------------
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential git wget unzip curl ca-certificates \
    libboost-all-dev libeigen3-dev libtbb-dev libflann-dev \
    libpcap-dev libgdal-dev libproj-dev libspatialindex-dev \
    postgresql-client \
    && rm -rf /var/lib/apt/lists/*

# -----------------------------
# Install Go
# -----------------------------
RUN wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -O /tmp/go.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz
ENV GOROOT="/usr/local/go"
ENV PATH="${GOROOT}/bin:${PATH}"

# -----------------------------
# Install Java (Oracle JDK 25)
# -----------------------------
RUN wget -q https://download.oracle.com/java/${JAVA_VERSION}/latest/jdk-${JAVA_VERSION}_linux-x64_bin.tar.gz -O /tmp/jdk.tar.gz \
    && mkdir -p /usr/local/java \
    && tar -xzf /tmp/jdk.tar.gz -C /usr/local/java --strip-components=1 \
    && rm /tmp/jdk.tar.gz
ENV JAVA_HOME="/usr/local/java"
ENV PATH="${JAVA_HOME}/bin:${PATH}"

# -----------------------------
# Install citydb-tool
# -----------------------------
RUN wget -q https://github.com/3dcitydb/citydb-tool/releases/download/v1.1.0/citydb-tool-1.1.0.zip -O /tmp/citydb-tool.zip \
    && unzip /tmp/citydb-tool.zip -d /usr/local/citydb-tool \
    && rm /tmp/citydb-tool.zip

# -----------------------------
# Copy and build project (as root)
# -----------------------------
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o /usr/local/bin/city2tabula ./cmd

# -----------------------------
# Create non-root user and fix permissions
# -----------------------------
RUN useradd -m -u 10001 appuser \
    && mkdir -p /home/appuser/go/bin \
    && chown -R appuser:appuser /home/appuser /app
ENV GOPATH="/home/appuser/go"
ENV PATH="${GOPATH}/bin:${PATH}"

USER appuser

# -----------------------------
# Set up shell environment
# -----------------------------
RUN echo 'export GOROOT=/usr/local/go' >> /home/appuser/.bashrc \
    && echo 'export GOPATH=/home/appuser/go' >> /home/appuser/.bashrc \
    && echo 'export JAVA_HOME=/usr/local/java' >> /home/appuser/.bashrc \
    && echo 'export PATH=$GOROOT/bin:$GOPATH/bin:$JAVA_HOME/bin:$PATH' >> /home/appuser/.bashrc

# -----------------------------
# Default command
# -----------------------------
CMD ["bash", "-lc", "echo 'Go:' $(go version); echo 'Java:' $(java -version 2>&1 | head -n 1); echo 'citydb-tool:' $(/usr/local/citydb-tool/citydb --version); exec bash"]
