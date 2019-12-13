#!/usr/bin/env bash
cd "$(dirname "$0")"
source _env/bin/activate
python src/www.py
