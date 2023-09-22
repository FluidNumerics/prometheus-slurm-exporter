# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0
FROM golang:1.20-alpine

RUN go install github.com/rivosinc/prometheus-slurm-exporter@latest
ENTRYPOINT ["prometheus-slurm-exporter"]
