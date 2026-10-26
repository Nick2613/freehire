#!/bin/sh
set -eu

# Run these sequentially: Meilisearch has one task queue.
/app/reindex
/app/reindex-companies
