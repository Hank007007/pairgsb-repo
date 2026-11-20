#!/usr/bin/env node
'use strict';

/**
 * Walk the fixtures directory and print what the intake layer makes of each
 * file: the header, the number of data records, a few decoded cells, and the
 * cells as they would be written back out.
 */

const fs = require('node:fs');
const path = require('node:path');

const { parseSheet } = require('../src');
const { sniff } = require('../src/dialect');
const { exportCell } = require('../src/cells');

const FIXTURES = path.join(__dirname, '..', 'fixtures');
const PREVIEW_ROWS = 3;
const CELL_WIDTH = 120;
const HEADER_WIDTH = 200;

/**
 * Columns the sales workflow marks as numeric or dates. Matched by header name
 * so a fixture does not need a schema file of its own.
 */
const NUMBER_COLUMNS = new Set(['seats', 'mrr', 'headcount', 'weight', 'amount', 'booked']);
const DATE_COLUMNS = new Set(['renewal', 'renewed_on']);

function typesFor(header) {
  const types = {};
  for (const name of header || []) {
    if (NUMBER_COLUMNS.has(name)) {
      types[name] = 'number';
    } else if (DATE_COLUMNS.has(name)) {
      types[name] = 'date';
    }
  }
  return types;
}

function previewRows(rows) {
  return rows.slice(0, PREVIEW_ROWS);
}

function truncate(text, limit) {
  return text.length > limit ? text.slice(0, limit) + '…' : text;
}

/**
 * Make invisible characters visible in the terminal: BOMs, stray carriage
 * returns and tabs are common in exported spreadsheets and are hard to spot in
 * raw output.
 */
function visible(text) {
  return text.replace(
    /[\u0000-\u001f\u007f﻿]/g,
    (ch) => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'),
  );
}

function show(value) {
  return truncate(visible(JSON.stringify(value)), HEADER_WIDTH);
}

function describeError(error) {
  if (!error) {
    return 'none';
  }
  return `${error.code} at row ${error.row}, column ${error.column}: ${error.message}`;
}

function describeCell(value) {
  const text =
    value instanceof Date ? value.toISOString() : visible(exportCell(value));
  return `[${truncate(text, CELL_WIDTH)}]`;
}

function report(name) {
  const fullPath = path.join(FIXTURES, name);
  const bytes = fs.readFileSync(fullPath);
  const dialect = sniff(bytes, { filename: name });
  const probe = parseSheet(bytes, { filename: name });
  const result = parseSheet(bytes, {
    filename: name,
    types: typesFor(probe.header),
  });

  process.stdout.write(`  delimiter: ${show(dialect.delimiter)}\n`);
  process.stdout.write(`  header: ${show(result.header)}\n`);
  process.stdout.write(`  rows: ${result.rows.length}\n`);
  process.stdout.write(`  error: ${describeError(result.error)}\n`);

  for (const [index, row] of previewRows(result.rows).entries()) {
    const cells = Object.keys(row).map(
      (key) => `${visible(key)}=${describeCell(row[key])}`,
    );
    process.stdout.write(`  row ${index + 1}: ${cells.join(' ')}\n`);
  }
}

function main() {
  const files = fs
    .readdirSync(FIXTURES)
    .filter((name) => /\.(csv|tsv)$/i.test(name))
    .sort();

  if (files.length === 0) {
    process.stdout.write('no fixtures found\n');
    return;
  }

  for (const name of files) {
    process.stdout.write(`${name}\n`);
    try {
      report(name);
    } catch (error) {
      process.stdout.write(`  threw: ${error.name}: ${error.message}\n`);
    }
    process.stdout.write('\n');
  }
}

main();
