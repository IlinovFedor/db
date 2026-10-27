#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd "$DIR"

export TEXINPUTS="../../texmf/tex:"
export BSTINPUTS="../../texmf/bibtex/bst/gost:"

echo "==> 1/4: pdflatex (first pass)..."
pdflatex -shell-escape -interaction=nonstopmode report-lab-04.tex >/dev/null || true

echo "==> 2/4: bibtex..."
bibtex report-lab-04

echo "==> 3/4: pdflatex (second pass)..."
pdflatex -shell-escape -interaction=nonstopmode report-lab-04.tex >/dev/null

echo "==> 4/4: pdflatex (final pass)..."
pdflatex -shell-escape -interaction=nonstopmode report-lab-04.tex >/dev/null

echo "==> Success! PDF built at $DIR/report-lab-04.pdf"
