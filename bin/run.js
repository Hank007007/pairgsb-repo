#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');

const { compileRule } = require('../src/compile');
const { evaluate } = require('../src/eval');
const { isRuleError } = require('../src/errors');

const ROOT = path.join(__dirname, '..');
const RULES_DIR = path.join(ROOT, 'fixtures', 'rules');
const EVENTS_DIR = path.join(ROOT, 'fixtures', 'events');

function readJsonDir(dir) {
  return fs.readdirSync(dir)
    .filter((name) => name.endsWith('.json'))
    .sort()
    .map((name) => {
      const full = path.join(dir, name);
      return {
        file: name,
        name: name.replace(/\.json$/, ''),
        data: JSON.parse(fs.readFileSync(full, 'utf8')),
      };
    });
}

function main() {
  const rules = readJsonDir(RULES_DIR);
  const events = readJsonDir(EVENTS_DIR);

  console.log('rulemill fixtures');
  console.log('');

  for (const event of events) {
    console.log(`event: ${event.name}`);

    for (const rule of rules) {
      const id = `${rule.data.tenant}/${rule.data.name}`;
      let verdict;
      try {
        const compiled = compileRule(rule.data.expression, {
          tenantId: rule.data.tenant,
        });
        const hit = evaluate(compiled, event.data);
        verdict = hit ? 'hit' : 'miss';
      } catch (error) {
        verdict = isRuleError(error) ? `error: ${error.code}` : `error: ${error.message}`;
      }
      console.log(`  ${id.padEnd(40)} ${verdict}`);
    }

    console.log('');
  }
}

main();
