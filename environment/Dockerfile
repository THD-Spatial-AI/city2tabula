FROM debian:bookworm-slim

ARG DEBIAN_FRONTEND=noninteractive
ENV GO_VERSION=1.23.3
ENV JAVA_VERSION=25

# Install system dependencies (without Java)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    git \
    wget \
    unzip \
    curl \
    ca-certificates \
    libboost-all-dev \
    libeigen3-dev \
    libtbb-dev \
    libflann-dev \
    libpcap-dev \
    libgdal-dev \
    libproj-dev \
    libspatialindex-dev \
    postgresql-client \
    && rm -rf /var/lib/apt/lists/*

# -----------------------------
# Install Go
# -----------------------------
RUN wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -O /tmp/go.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

# -----------------------------
# Install Java 25 from official OpenJDK
# -----------------------------
RUN wget -q https://download.oracle.com/java/${JAVA_VERSION}/latest/jdk-${JAVA_VERSION}_linux-x64_bin.tar.gz -O /tmp/jdk.tar.gz \
    && rm -rf /usr/local/java \
    && mkdir -p /usr/local/java \
    && tar -C /usr/local/java -xzf /tmp/jdk.tar.gz --strip-components=1 \
    && rm /tmp/jdk.tar.gz

# -----------------------------
# Install citydb-tool
# -----------------------------
RUN wget -q https://github.com/3dcitydb/citydb-tool/releases/download/v1.1.0/citydb-tool-1.1.0.zip -O /tmp/citydb-tool.zip \
    && unzip /tmp/citydb-tool.zip -d /usr/local/citydb-tool \
    && rm /tmp/citydb-tool.zip

# Set environment variables globally
ENV JAVA_HOME="/usr/local/java"
ENV GOROOT="/usr/local/go"
ENV GOPATH="/home/appuser/go"
ENV PATH="${GOROOT}/bin:${GOPATH}/bin:${JAVA_HOME}/bin:${PATH}"

# -----------------------------
# Create non-root user
# -----------------------------
RUN useradd -m -u 10001 appuser \
    && mkdir -p /home/appuser/go/bin \
    && chown -R appuser:appuser /home/appuser

# Create app directory and set ownership
RUN mkdir -p /app && chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Copy application code
WORKDIR /app
COPY . .

# Remove old .env file
RUN rm -rf .env

# Set up user environment
RUN echo 'export GOROOT=/usr/local/go' >> /home/appuser/.bashrc \
    && echo 'export GOPATH=/home/appuser/go' >> /home/appuser/.bashrc \
    && echo 'export JAVA_HOME=/usr/local/java' >> /home/appuser/.bashrc \
    && echo 'export PATH=$GOROOT/bin:$GOPATH/bin:$JAVA_HOME/bin:$PATH' >> /home/appuser/.bashrc

# -----------------------------
# Build the Go application
# -----------------------------
RUN rm -rf city2tabula
RUN go build -buildvcs=false -o city2tabula ./cmd/main.go

# -----------------------------
# Default command
# -----------------------------
CMD ["bash", "-lc", "echo 'Go:' $(go version); echo 'Java:' $(java -version 2>&1 | head -n 1); echo 'citydb-tool:' $(/usr/local/citydb-tool/citydb --version); exec bash"]