/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

const BILLING_VAR_MAP = {
  p: 'input_price',
  c: 'output_price',
  cr: 'cache_read_price',
  cc: 'cache_create_price',
  cc1h: 'cache_create_1h_price',
  img: 'image_price',
  img_o: 'image_output_price',
  ai: 'audio_input_price',
  ao: 'audio_output_price',
};

const BILLING_VAR_REGEX = new RegExp(
  `\\b(${Object.keys(BILLING_VAR_MAP).join('|')})\\s*\\*\\s*([\\d.eE+-]+)`,
  'g',
);

function stripExprVersion(exprStr) {
  if (!exprStr) return { version: 1, body: '' };
  const match = exprStr.match(/^v(\d+):([\s\S]*)$/);
  if (match) return { version: Number(match[1]), body: match[2] };
  return { version: 1, body: exprStr };
}

function parseTierBody(bodyStr) {
  const coeffs = {};
  const re = new RegExp(BILLING_VAR_REGEX.source, 'g');
  let match;
  while ((match = re.exec(bodyStr)) !== null) {
    if (!(match[1] in coeffs)) coeffs[match[1]] = Number(match[2]);
  }
  const tier = {};
  Object.entries(BILLING_VAR_MAP).forEach(([varName, field]) => {
    tier[field] = coeffs[varName] || 0;
  });
  return tier;
}

export function parseTiersFromExpr(exprStr) {
  if (!exprStr) return [];
  try {
    const { body } = stripExprVersion(exprStr);
    const condGroup =
      `((?:(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)` +
      `(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)*)`;
    const tierRe = new RegExp(
      `(?:${condGroup}\\s*\\?\\s*)?tier\\("([^"]*)",\\s*([^)]+)\\)`,
      'g',
    );
    const tiers = [];
    let match;
    while ((match = tierRe.exec(body)) !== null) {
      const condStr = match[1] || '';
      const conditions = [];
      if (condStr) {
        condStr.split(/\s*&&\s*/).forEach((conditionPart) => {
          const condMatch = conditionPart
            .trim()
            .match(/^(p|c|len)\s*(<|<=|>|>=)\s*([\d.eE+]+)$/);
          if (condMatch) {
            conditions.push({
              var: condMatch[1],
              op: condMatch[2],
              value: Number(condMatch[3]),
            });
          }
        });
      }
      const tier = parseTierBody(match[3]);
      tier.label = match[2];
      tier.conditions = conditions;
      tiers.push(tier);
    }
    return tiers;
  } catch {
    return [];
  }
}
