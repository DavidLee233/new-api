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

import React from 'react';
import {
  Avatar,
  Space,
  Tag,
  Tooltip,
  Popover,
  Typography,
} from '@douyinfe/semi-ui';
import {
  renderGroup,
  renderQuota,
  stringToColor,
  getLogOther,
  renderModelTag,
  renderModelPriceSimple,
} from '../../../helpers';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import { CircleAlert, Route, Sparkles } from 'lucide-react';

const colors = [
  'amber',
  'blue',
  'cyan',
  'green',
  'grey',
  'indigo',
  'light-blue',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
];

function formatRatio(ratio) {
  if (ratio === undefined || ratio === null) {
    return '-';
  }
  if (typeof ratio === 'number') {
    return ratio.toFixed(4);
  }
  return String(ratio);
}

function formatFormulaNumber(value, digits = 4) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return '0';
  }
  return parsed
    .toFixed(digits)
    .replace(/\.?0+$/, '');
}

function getQuotaPerUnitValue() {
  try {
    const raw = localStorage.getItem('quota_per_unit');
    const parsed = Number(raw);
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed;
    }
  } catch (error) {}
  return 500000;
}

function buildQuotaFormulaLine(label, formula, amount) {
  return {
    label,
    formula,
    amount,
  };
}

function buildChannelAffinityTooltip(affinity, t) {
  if (!affinity) {
    return null;
  }

  const keySource = affinity.key_source || '-';
  const keyPath = affinity.key_path || affinity.key_key || '-';
  const keyHint = affinity.key_hint || '';
  const keyFp = affinity.key_fp ? `#${affinity.key_fp}` : '';
  const keyText = `${keySource}:${keyPath}${keyFp}`;

  const lines = [
    t('\u6e20\u9053\u4eb2\u548c'),
    `${t('\u89c4\u5219')}: ${affinity.rule_name || '-'}` ,
    `${t('\u5206\u7ec4')}: ${affinity.selected_group || '-'}` ,
    `${t('Key')}: ${keyText}` ,
    ...(keyHint ? [`${t('Key \u6458\u8981')}: ${keyHint}`] : []),
  ];

  return (
    <div style={{ lineHeight: 1.6, display: 'flex', flexDirection: 'column' }}>
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  );
}

// Render functions
function renderType(type, t) {
  switch (type) {
    case 1:
      return (
        <Tag color='cyan' shape='circle'>
          {t('\u5145\u503c')}
        </Tag>
      );
    case 2:
      return (
        <Tag color='lime' shape='circle'>
          {t('\u6d88\u8d39')}
        </Tag>
      );
    case 3:
      return (
        <Tag color='orange' shape='circle'>
          {t('\u7ba1\u7406')}
        </Tag>
      );
    case 4:
      return (
        <Tag color='purple' shape='circle'>
          {t('\u7cfb\u7edf')}
        </Tag>
      );
    case 5:
      return (
        <Tag color='red' shape='circle'>
          {t('\u9519\u8bef')}
        </Tag>
      );
    case 6:
      return (
        <Tag color='teal' shape='circle'>
          {t('\u9000\u8d39')}
        </Tag>
      );
    default:
      return (
        <Tag color='grey' shape='circle'>
          {t('\u672a\u77e5')}
        </Tag>
      );
  }
}

function buildStreamStatusTooltip(ss, t) {
  if (!ss) return null;
  const lines = [
    t('\u6d41\u72b6\u6001') + ' / ' + t('\u5f02\u5e38'),
    ss.end_reason || 'unknown',
  ];
  if (ss.error_count > 0) {
    lines.push(`${t('\u8f6f\u9519')}: ${ss.error_count}`);
  }
  if (ss.end_error) {
    lines.push(ss.end_error);
  }
  return (
    <div style={{ lineHeight: 1.6, display: 'flex', flexDirection: 'column' }}>
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  );
}

function renderIsStream(bool, t, streamStatus) {
  const isError = streamStatus && streamStatus.status !== 'ok';

  if (bool) {
    return (
      <span style={{ position: 'relative', display: 'inline-block' }}>
        <Tag color='blue' shape='circle'>
          {t('\u6d41')}
        </Tag>
        {isError && (
          <Tooltip content={buildStreamStatusTooltip(streamStatus, t)}>
            <span
              style={{
                position: 'absolute',
                right: -4,
                top: -4,
                lineHeight: 1,
                color: '#ef4444',
                cursor: 'pointer',
                userSelect: 'none',
              }}
            >
              <CircleAlert size={12} />
            </span>
          </Tooltip>
        )}
      </span>
    );
  }
  return <Tag color='grey' shape='circle'>{t('\u975e\u6d41')}</Tag>;
}

function renderUseTime(type, t) {
  const time = parseInt(type);
  if (time < 101) {
    return (
      <Tag color='green' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else if (time < 300) {
    return (
      <Tag color='orange' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else {
    return (
      <Tag color='red' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  }
}

function renderFirstUseTime(type, t) {
  let time = parseFloat(type) / 1000.0;
  time = time.toFixed(1);
  if (time < 3) {
    return (
      <Tag color='green' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else if (time < 10) {
    return (
      <Tag color='orange' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else {
    return (
      <Tag color='red' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  }
}

function renderBillingTag(record, t) {
  const other = getLogOther(record.other);
  if (other?.billing_source === 'subscription') {
    return (
      <Tag color='green' shape='circle'>
        {t('\u8ba2\u9605\u62b5\u6263')}
      </Tag>
    );
  }
  return null;
}

function renderModelName(record, copyText, t) {
  let other = getLogOther(record.other);
  let modelMapped =
    other?.is_model_mapped &&
    other?.upstream_model_name &&
    other?.upstream_model_name !== '';
  if (!modelMapped) {
    return renderModelTag(record.model_name, {
      onClick: (event) => {
        copyText(event, record.model_name).then(() => {});
      },
    });
  } else {
    return (
      <>
        <Space vertical align={'start'}>
          <Popover
            content={
              <div style={{ padding: 10 }}>
                <Space vertical align={'start'}>
                  <div className='flex items-center'>
                    <Typography.Text strong style={{ marginRight: 8 }}>
                      {t('\u8bf7\u6c42\u5e76\u8ba1\u8d39\u6a21\u578b')}:
                    </Typography.Text>
                    {renderModelTag(record.model_name, {
                      onClick: (event) => {
                        copyText(event, record.model_name).then(() => {});
                      },
                    })}
                  </div>
                  <div className='flex items-center'>
                    <Typography.Text strong style={{ marginRight: 8 }}>
                      {t('\u5b9e\u9645\u6a21\u578b')}:
                    </Typography.Text>
                    {renderModelTag(other.upstream_model_name, {
                      onClick: (event) => {
                        copyText(event, other.upstream_model_name).then(
                          () => {},
                        );
                      },
                    })}
                  </div>
                </Space>
              </div>
            }
          >
            {renderModelTag(record.model_name, {
              onClick: (event) => {
                copyText(event, record.model_name).then(() => {});
              },
              suffixIcon: (
                <Route
                  style={{ width: '0.9em', height: '0.9em', opacity: 0.75 }}
                />
              ),
            })}
          </Popover>
        </Space>
      </>
    );
  }
}

function toTokenNumber(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return parsed;
}

function formatTokenCount(value) {
  return toTokenNumber(value).toLocaleString();
}

function getPromptCacheSummary(other) {
  if (!other || typeof other !== 'object') {
    return null;
  }

  const cacheReadTokens = toTokenNumber(other.cache_tokens);
  const explicitCacheWriteTokens = toTokenNumber(other.cache_write_tokens);
  const cacheCreationTokens = toTokenNumber(other.cache_creation_tokens);
  const cacheCreationTokens5m = toTokenNumber(other.cache_creation_tokens_5m);
  const cacheCreationTokens1h = toTokenNumber(other.cache_creation_tokens_1h);

  const hasSplitCacheCreation =
    cacheCreationTokens5m > 0 || cacheCreationTokens1h > 0;
  const fallbackCacheWriteTokens = hasSplitCacheCreation
    ? cacheCreationTokens5m + cacheCreationTokens1h
    : cacheCreationTokens;
  const cacheWriteTokens =
    explicitCacheWriteTokens > 0
      ? explicitCacheWriteTokens
      : fallbackCacheWriteTokens;

  if (cacheReadTokens <= 0 && cacheWriteTokens <= 0) {
    return null;
  }

  return {
    cacheReadTokens,
    cacheWriteTokens,
  };
}

function getInputTokenDisplay(record, other) {
  const promptTokens = toTokenNumber(record?.prompt_tokens);
  const cacheSummary = getPromptCacheSummary(other);
  const cacheReadTokens = cacheSummary?.cacheReadTokens || 0;
  const cacheWriteTokens = cacheSummary?.cacheWriteTokens || 0;
  const explicitTotal = toTokenNumber(other?.input_tokens_total);

  if (explicitTotal > 0) {
    return {
      promptText: formatTokenCount(explicitTotal),
      cacheReadTokens,
      cacheWriteTokens,
    };
  }

  if (cacheReadTokens > 0 || cacheWriteTokens > 0) {
    return {
      promptText: formatTokenCount(
        promptTokens + cacheReadTokens + cacheWriteTokens,
      ),
      cacheReadTokens,
      cacheWriteTokens,
    };
  }

  return {
    promptText: formatTokenCount(promptTokens),
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
  };
}

function normalizeDetailText(detail) {
  return String(detail || '')
    .replace(/\n\r/g, '\n')
    .replace(/\r\n/g, '\n');
}

function getUsageLogGroupSummary(groupRatio, userGroupRatio, t) {
  const parsedUserGroupRatio = Number(userGroupRatio);
  const useUserGroupRatio =
    Number.isFinite(parsedUserGroupRatio) && parsedUserGroupRatio !== -1;
  const ratio = useUserGroupRatio ? userGroupRatio : groupRatio;
  if (ratio === undefined || ratio === null || ratio === '') {
    return '';
  }
  return `${useUserGroupRatio ? t('专属倍率') : t('分组')} ${formatRatio(ratio)}x`;
}

function getEffectiveGroupRatioInfo(groupRatio, userGroupRatio, t) {
  const parsedUserGroupRatio = Number(userGroupRatio);
  const useUserGroupRatio =
    Number.isFinite(parsedUserGroupRatio) && parsedUserGroupRatio !== -1;
  const value = Number(useUserGroupRatio ? userGroupRatio : groupRatio);
  return {
    value: Number.isFinite(value) ? value : 1,
    label: useUserGroupRatio ? t('专属倍率') : t('分组倍率'),
  };
}

function getCacheWriteFormulaParts(other) {
  const explicitCacheWriteTokens = toTokenNumber(other?.cache_write_tokens);
  const cacheCreationTokens = toTokenNumber(other?.cache_creation_tokens);
  const cacheCreationTokens5m = toTokenNumber(other?.cache_creation_tokens_5m);
  const cacheCreationTokens1h = toTokenNumber(other?.cache_creation_tokens_1h);
  const hasSplit = cacheCreationTokens5m > 0 || cacheCreationTokens1h > 0;

  if (hasSplit) {
    return [
      cacheCreationTokens5m > 0
        ? {
            label: '缓存写(5m)',
            tokens: cacheCreationTokens5m,
            ratio: Number(other?.cache_creation_ratio_5m || other?.cache_creation_ratio || 1),
          }
        : null,
      cacheCreationTokens1h > 0
        ? {
            label: '缓存写(1h)',
            tokens: cacheCreationTokens1h,
            ratio: Number(other?.cache_creation_ratio_1h || other?.cache_creation_ratio || 1),
          }
        : null,
      cacheCreationTokens > cacheCreationTokens5m + cacheCreationTokens1h
        ? {
            label: '缓存写',
            tokens:
              cacheCreationTokens - cacheCreationTokens5m - cacheCreationTokens1h,
            ratio: Number(other?.cache_creation_ratio || 1),
          }
        : null,
    ].filter(Boolean);
  }

  const fallbackTokens =
    explicitCacheWriteTokens > 0 ? explicitCacheWriteTokens : cacheCreationTokens;
  if (fallbackTokens <= 0) {
    return [];
  }

  return [
    {
      label: '缓存写',
      tokens: fallbackTokens,
      ratio: Number(other?.cache_creation_ratio || 1),
    },
  ];
}

function getPrimaryCacheWriteRatio(other) {
  const ratio5m = Number(other?.cache_creation_ratio_5m || 0);
  if (Number.isFinite(ratio5m) && ratio5m > 0) {
    return ratio5m;
  }
  const ratio1h = Number(other?.cache_creation_ratio_1h || 0);
  if (Number.isFinite(ratio1h) && ratio1h > 0) {
    return ratio1h;
  }
  const ratio = Number(other?.cache_creation_ratio || 0);
  if (Number.isFinite(ratio) && ratio > 0) {
    return ratio;
  }
  return 5;
}

function renderUsageLogFormulaContent(detailSummary, t) {
  const formulaLines = Array.isArray(detailSummary?.formulaLines)
    ? detailSummary.formulaLines
    : [];
  const notes = Array.isArray(detailSummary?.notes) ? detailSummary.notes : [];

  if (!formulaLines.length && !notes.length) {
    return null;
  }

  return (
    <div
      style={{
        width: 460,
        maxWidth: '80vw',
        maxHeight: '60vh',
        overflowY: 'auto',
        lineHeight: 1.6,
      }}
    >
      <div style={{ fontWeight: 600, marginBottom: 8 }}>{t('花费公式')}</div>
      {formulaLines.map((line, index) => (
        <div
          key={`${line.label}-${index}`}
          style={{
            padding: '8px 0',
            borderTop: index === 0 ? 'none' : '1px solid var(--semi-color-border)',
          }}
        >
          <div style={{ fontWeight: 500 }}>{line.label}</div>
          <div style={{ fontSize: 12, color: 'var(--semi-color-text-1)' }}>
            {line.formula}
          </div>
          {line.amount ? (
            <div style={{ fontSize: 12, color: 'var(--semi-color-text-2)', marginTop: 2 }}>
              = {line.amount}
            </div>
          ) : null}
        </div>
      ))}
      {notes.length ? (
        <div
          style={{
            marginTop: 8,
            paddingTop: 8,
            borderTop: '1px solid var(--semi-color-border)',
            fontSize: 12,
            color: 'var(--semi-color-text-2)',
          }}
        >
          {notes.map((note, index) => (
            <div key={`${note}-${index}`}>{note}</div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function buildUsageLogDetailFormula(record, other, text, t) {
  if (record.type === 6) {
    return {
      formulaLines: [
        buildQuotaFormulaLine(
          t('异步任务退款'),
          t('本条日志为退款记录，表示返还此前已扣除的额度'),
          renderQuota(record?.quota, 6),
        ),
      ],
    };
  }

  if (
    other?.violation_fee === true ||
    Boolean(other?.violation_fee_code) ||
    Boolean(other?.violation_fee_marker)
  ) {
    const feeQuota = Number(other?.fee_quota ?? record?.quota ?? 0);
    const groupInfo = getEffectiveGroupRatioInfo(
      other?.group_ratio,
      other?.user_group_ratio,
      t,
    );
    return {
      formulaLines: [
        buildQuotaFormulaLine(
          t('违规扣费'),
          `${t('固定扣费')} ${renderQuota(feeQuota, 6)}${
            groupInfo ? `，${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x` : ''
          }`,
          renderQuota(record?.quota, 6),
        ),
      ],
      notes: text ? [`${t('详情')}: ${text}`] : [],
    };
  }

  if (record.type !== 2) {
    return null;
  }

  const promptTokens = toTokenNumber(record?.prompt_tokens);
  const completionTokens = toTokenNumber(record?.completion_tokens);
  const modelRatio = Number(other?.model_ratio || 0);
  const completionRatio = Number(other?.completion_ratio || 1);
  const cacheReadTokens = toTokenNumber(other?.cache_tokens);
  const cacheRatio = Number(other?.cache_ratio || 1);
  const primaryCacheWriteRatio = getPrimaryCacheWriteRatio(other);
  const groupInfo = getEffectiveGroupRatioInfo(
    other?.group_ratio,
    other?.user_group_ratio,
    t,
  );
  const quotaPerUnit = getQuotaPerUnitValue();
  const formulaLines = [];
  const notes = [t('说明：以下为根据日志字段还原的可读公式，最终以实际扣费为准。')];

  if (Number(other?.model_price) > -1) {
    const modelPrice = Number(other?.model_price || 0);
    const fixedQuota = modelPrice * quotaPerUnit * groupInfo.value;
    formulaLines.push(
      buildQuotaFormulaLine(
        t('按次模型费用'),
        `${t('模型单价')} ${renderQuota(modelPrice * quotaPerUnit, 6)} × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
        renderQuota(fixedQuota, 6),
      ),
    );
  } else {
    if (promptTokens > 0) {
      const cacheWriteQuota =
        promptTokens * modelRatio * primaryCacheWriteRatio * groupInfo.value;
      formulaLines.push(
        buildQuotaFormulaLine(
          t('缓存写'),
          `${promptTokens} tokens × ${t('模型倍率')} ${formatFormulaNumber(modelRatio)} × ${t('缓存写倍率')} ${formatFormulaNumber(primaryCacheWriteRatio)} × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
          renderQuota(cacheWriteQuota, 6),
        ),
      );
    }

    if (cacheReadTokens > 0) {
      const cacheReadQuota =
        cacheReadTokens * modelRatio * cacheRatio * groupInfo.value;
      formulaLines.push(
        buildQuotaFormulaLine(
          t('缓存读'),
          `${cacheReadTokens} tokens × ${t('模型倍率')} ${formatFormulaNumber(modelRatio)} × ${t('缓存读倍率')} ${formatFormulaNumber(cacheRatio)} × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
          renderQuota(cacheReadQuota, 6),
        ),
      );
    }

    if (promptTokens <= 0) {
      const cacheWriteParts = getCacheWriteFormulaParts(other);
      cacheWriteParts.forEach((part) => {
        const cacheWriteQuota =
          part.tokens * modelRatio * Number(part.ratio || 1) * groupInfo.value;
        formulaLines.push(
          buildQuotaFormulaLine(
            t(part.label),
            `${part.tokens} tokens × ${t('模型倍率')} ${formatFormulaNumber(modelRatio)} × ${t('缓存写倍率')} ${formatFormulaNumber(part.ratio)} × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
            renderQuota(cacheWriteQuota, 6),
          ),
        );
      });
    }

    if (completionTokens > 0) {
      const completionQuota =
        completionTokens * modelRatio * completionRatio * groupInfo.value;
      formulaLines.push(
        buildQuotaFormulaLine(
          t('输出'),
          `${completionTokens} tokens × ${t('模型倍率')} ${formatFormulaNumber(modelRatio)} × ${t('输出倍率')} ${formatFormulaNumber(completionRatio)} × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
          renderQuota(completionQuota, 6),
        ),
      );
    }
  }

  const webSearchCallCount = toTokenNumber(other?.web_search_call_count);
  const webSearchPrice = Number(other?.web_search_price || 0);
  if (webSearchCallCount > 0 && webSearchPrice > 0) {
    const webSearchQuota =
      (webSearchPrice * webSearchCallCount * quotaPerUnit * groupInfo.value) /
      1000;
    formulaLines.push(
      buildQuotaFormulaLine(
        t('Web 搜索'),
        `${webSearchCallCount} ${t('次')} × ${renderQuota(webSearchPrice * quotaPerUnit, 6)} / 1000 × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
        renderQuota(webSearchQuota, 6),
      ),
    );
  }

  const fileSearchCallCount = toTokenNumber(other?.file_search_call_count);
  const fileSearchPrice = Number(other?.file_search_price || 0);
  if (fileSearchCallCount > 0 && fileSearchPrice > 0) {
    const fileSearchQuota =
      (fileSearchPrice * fileSearchCallCount * quotaPerUnit * groupInfo.value) /
      1000;
    formulaLines.push(
      buildQuotaFormulaLine(
        t('File 搜索'),
        `${fileSearchCallCount} ${t('次')} × ${renderQuota(fileSearchPrice * quotaPerUnit, 6)} / 1000 × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
        renderQuota(fileSearchQuota, 6),
      ),
    );
  }

  if (other?.image_generation_call && Number(other?.image_generation_call_price) > 0) {
    const imageGenerationPrice = Number(other?.image_generation_call_price || 0);
    const imageGenerationQuota =
      imageGenerationPrice * quotaPerUnit * groupInfo.value;
    formulaLines.push(
      buildQuotaFormulaLine(
        t('绘图调用'),
        `${renderQuota(imageGenerationPrice * quotaPerUnit, 6)} × ${groupInfo.label} ${formatFormulaNumber(groupInfo.value)}x`,
        renderQuota(imageGenerationQuota, 6),
      ),
    );
  }

  if (Number(other?.audio_input_price) > 0) {
    notes.push(
      `${t('提示')}: ${t('该请求包含音频输入计费，音频部分已计入最终花费。')}`,
    );
  }

  notes.push(t('当前公式口径：缓存写 + 缓存读 + 输出 + 附加项。'));

  formulaLines.push(
    buildQuotaFormulaLine(
      t('最终花费'),
      t('以上各项合计'),
      renderQuota(record?.quota, 6),
    ),
  );

  return {
    formulaLines,
    notes,
  };
}

function renderCompactDetailSummary(summarySegments) {
  const segments = Array.isArray(summarySegments)
    ? summarySegments.filter((segment) => segment?.text)
    : [];
  if (!segments.length) {
    return null;
  }

  return (
    <div
      style={{
        maxWidth: 180,
        lineHeight: 1.35,
      }}
    >
      {segments.map((segment, index) => (
        <Typography.Text
          key={`${segment.text}-${index}`}
          type={segment.tone === 'secondary' ? 'tertiary' : undefined}
          size={segment.tone === 'secondary' ? 'small' : undefined}
          style={{
            display: 'block',
            maxWidth: '100%',
            fontSize: 12,
            marginTop: index === 0 ? 0 : 2,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {segment.text}
        </Typography.Text>
      ))}
    </div>
  );
}

function getUsageLogDetailSummary(record, text, billingDisplayMode, t) {
  const other = getLogOther(record.other);
  const formula = buildUsageLogDetailFormula(record, other, text, t);

  if (record.type === 6) {
    return {
      segments: [{ text: t('\u5f02\u6b65\u4efb\u52a1\u9000\u8d39'), tone: 'primary' }],
      formula,
    };
  }

  if (other == null || record.type !== 2) {
    return null;
  }

  if (
    other?.violation_fee === true ||
    Boolean(other?.violation_fee_code) ||
    Boolean(other?.violation_fee_marker)
  ) {
    const feeQuota = other?.fee_quota ?? record?.quota;
    const groupText = getUsageLogGroupSummary(
      other?.group_ratio,
      other?.user_group_ratio,
      t,
    );
    return {
      segments: [
        groupText ? { text: groupText, tone: 'primary' } : null,
        { text: t('\u8fdd\u89c4\u6263\u8d39'), tone: 'primary' },
        {
          text: `${t('\u6263\u8d39')}: ${renderQuota(feeQuota, 6)}`,
          tone: 'secondary',
        },
        text ? { text: `${t('\u8be6\u60c5')}: ${text}`, tone: 'secondary' } : null,
      ].filter(Boolean),
      formula,
    };
  }

  return {
    segments: other?.claude
      ? renderModelPriceSimple(
          other.model_ratio,
          other.model_price,
          other.group_ratio,
          other?.user_group_ratio,
          other.cache_tokens || 0,
          other.cache_ratio || 1.0,
          other.cache_creation_tokens || 0,
          other.cache_creation_ratio || 1.0,
          other.cache_creation_tokens_5m || 0,
          other.cache_creation_ratio_5m || other.cache_creation_ratio || 1.0,
          other.cache_creation_tokens_1h || 0,
          other.cache_creation_ratio_1h || other.cache_creation_ratio || 1.0,
          false,
          1.0,
          other?.is_system_prompt_overwritten,
          'claude',
          billingDisplayMode,
          'segments',
        )
      : renderModelPriceSimple(
          other.model_ratio,
          other.model_price,
          other.group_ratio,
          other?.user_group_ratio,
          other.cache_tokens || 0,
          other.cache_ratio || 1.0,
          other.cache_creation_tokens || 0,
          other.cache_creation_ratio || 1.0,
          other.cache_creation_tokens_5m || 0,
          other.cache_creation_ratio_5m || other.cache_creation_ratio || 1.0,
          other.cache_creation_tokens_1h || 0,
          other.cache_creation_ratio_1h || other.cache_creation_ratio || 1.0,
          false,
          1.0,
          other?.is_system_prompt_overwritten,
          'openai',
          billingDisplayMode,
          'segments',
        ),
    formula,
  };
}
export const getLogsColumns = ({
  t,
  COLUMN_KEYS,
  copyText,
  showUserInfoFunc,
  openChannelAffinityUsageCacheModal,
  isAdminUser,
  billingDisplayMode = 'price',
}) => {
  return [
    {
      key: COLUMN_KEYS.TIME,
      title: t('时间'),
      dataIndex: 'timestamp2string',
    },
    {
      key: COLUMN_KEYS.CHANNEL,
      title: t('渠道'),
      dataIndex: 'channel',
      render: (text, record, index) => {
        let isMultiKey = false;
        let multiKeyIndex = -1;
        let content = t('\u6e20\u9053') + ': ' + record.channel;
        let affinity = null;
        let showMarker = false;
        let other = getLogOther(record.other);
        if (other?.admin_info) {
          let adminInfo = other.admin_info;
          if (adminInfo?.is_multi_key) {
            isMultiKey = true;
            multiKeyIndex = adminInfo.multi_key_index;
          }
          if (
            Array.isArray(adminInfo.use_channel) &&
            adminInfo.use_channel.length > 0
          ) {
            content = t('\u6e20\u9053') + ': ' + adminInfo.use_channel.join('->');
          }
          if (adminInfo.channel_affinity) {
            affinity = adminInfo.channel_affinity;
            showMarker = true;
          }
        }

        return isAdminUser &&
          (record.type === 0 ||
            record.type === 2 ||
            record.type === 5 ||
            record.type === 6) ? (
          <Space>
            <span style={{ position: 'relative', display: 'inline-block' }}>
              <Tooltip content={record.channel_name || t('未知渠道')}>
                <span>
                  <Tag
                    color={colors[parseInt(text) % colors.length]}
                    shape='circle'
                  >
                    {text}
                  </Tag>
                </span>
              </Tooltip>
              {showMarker && (
                <Tooltip
                  content={
                    <div style={{ lineHeight: 1.6 }}>
                      <div>{content}</div>
                      {affinity ? (
                        <div style={{ marginTop: 6 }}>
                          {buildChannelAffinityTooltip(affinity, t)}
                        </div>
                      ) : null}
                    </div>
                  }
                >
                  <span
                    style={{
                      position: 'absolute',
                      right: -4,
                      top: -4,
                      lineHeight: 1,
                      fontWeight: 600,
                      color: '#f59e0b',
                      cursor: 'pointer',
                      userSelect: 'none',
                    }}
                    onClick={(e) => {
                      e.stopPropagation();
                      openChannelAffinityUsageCacheModal?.(affinity);
                    }}
                  >
                    <Sparkles
                      size={14}
                      strokeWidth={2}
                      color='currentColor'
                      fill='currentColor'
                    />
                  </span>
                </Tooltip>
              )}
            </span>
            {isMultiKey && (
              <Tag color='white' shape='circle'>
                {multiKeyIndex}
              </Tag>
            )}
          </Space>
        ) : null;
      },
    },
    {
      key: COLUMN_KEYS.USERNAME,
      title: t('用户'),
      dataIndex: 'username',
      render: (text, record, index) => {
        return isAdminUser ? (
          <div>
            <Avatar
              size='extra-small'
              color={stringToColor(text)}
              style={{ marginRight: 4 }}
              onClick={(event) => {
                event.stopPropagation();
                showUserInfoFunc(record.user_id);
              }}
            >
              {typeof text === 'string' && text.slice(0, 1)}
            </Avatar>
            {text}
          </div>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.TOKEN,
      title: t('令牌'),
      dataIndex: 'token_name',
      render: (text, record, index) => {
        return record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6 ? (
          <div>
            <Tag
              color='grey'
              shape='circle'
              onClick={(event) => {
                copyText(event, text);
              }}
            >
              {' '}
              {t(text)}{' '}
            </Tag>
          </div>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.GROUP,
      title: t('分组'),
      dataIndex: 'group',
      render: (text, record, index) => {
        if (
          record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6
        ) {
          if (record.group) {
            return <>{renderGroup(record.group)}</>;
          } else {
            let other = null;
            try {
              other = JSON.parse(record.other);
            } catch (e) {
              console.error(
                `Failed to parse record.other: "${record.other}".`,
                e,
              );
            }
            if (other === null) {
              return <></>;
            }
            if (other.group !== undefined) {
              return <>{renderGroup(other.group)}</>;
            } else {
              return <></>;
            }
          }
        } else {
          return <></>;
        }
      },
    },
    {
      key: COLUMN_KEYS.TYPE,
      title: t('类型'),
      dataIndex: 'type',
      render: (text, record, index) => {
        return <>{renderType(text, t)}</>;
      },
    },
    {
      key: COLUMN_KEYS.MODEL,
      title: t('模型'),
      dataIndex: 'model_name',
      render: (text, record, index) => {
        return record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6 ? (
          <>{renderModelName(record, copyText, t)}</>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.USE_TIME,
      title: t('用时/首字'),
      dataIndex: 'use_time',
      render: (text, record, index) => {
        if (!(record.type === 2 || record.type === 5)) {
          return <></>;
        }
        if (record.is_stream) {
          let other = getLogOther(record.other);
          return (
            <>
              <Space>
                {renderUseTime(text, t)}
                {renderFirstUseTime(other?.frt, t)}
                {renderIsStream(record.is_stream, t, other?.stream_status)}
              </Space>
            </>
          );
        } else {
          return (
            <>
              <Space>
                {renderUseTime(text, t)}
                {renderIsStream(record.is_stream, t)}
              </Space>
            </>
          );
        }
      },
    },
    {
      key: COLUMN_KEYS.PROMPT,
      title: (
        <div className='flex items-center gap-1'>
          {t('输入')}
          <Tooltip
            content={t(
              '\u6839\u636e Anthropic \u534f\u5b9a v1/messages \u7684\u8f93\u5165 tokens \u4ec5\u7edf\u8ba1\u975e\u7f13\u5b58\u8f93\u5165\uff0c\u4e0d\u5305\u542b\u7f13\u5b58\u8bfb\u53d6\u4e0e\u7f13\u5b58\u5199\u5165 tokens',
            )}
          >
            <IconHelpCircle className='text-gray-400 cursor-help' />
          </Tooltip>
        </div>
      ),
      dataIndex: 'prompt_tokens',
      render: (text, record, index) => {
        const other = getLogOther(record.other);
        const { promptText, cacheReadTokens, cacheWriteTokens } =
          getInputTokenDisplay(record, other);

        const cacheParts = [];
        if (cacheReadTokens > 0) {
          cacheParts.push(`${t('\u7f13\u5b58\u8bfb')} ${formatTokenCount(cacheReadTokens)}`);
        }
        if (cacheWriteTokens > 0) {
          cacheParts.push(`${t('\u7f13\u5b58\u5199')} ${formatTokenCount(cacheWriteTokens)}`);
        }
        const cacheText = cacheParts.join('\uff0c');

        return record.type === 0 ||
          record.type === 2 ||
          record.type === 5 ||
          record.type === 6 ? (
          <div
            style={{
              display: 'inline-flex',
              flexDirection: 'column',
              alignItems: 'flex-start',
              lineHeight: 1.2,
            }}
          >
            <span>{promptText}</span>
            {cacheText ? (
              <span
                style={{
                  marginTop: 2,
                  fontSize: 11,
                  color: 'var(--semi-color-text-2)',
                  whiteSpace: 'nowrap',
                }}
              >
                {cacheText}
              </span>
            ) : null}
          </div>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.COMPLETION,
      title: t('输出'),
      dataIndex: 'completion_tokens',
      render: (text, record, index) => {
        return parseInt(text) > 0 &&
          (record.type === 0 ||
            record.type === 2 ||
            record.type === 5 ||
            record.type === 6) ? (
          <>{<span> {text} </span>}</>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.COST,
      title: t('花费'),
      dataIndex: 'quota',
      render: (text, record, index) => {
        if (
          !(
            record.type === 0 ||
            record.type === 2 ||
            record.type === 5 ||
            record.type === 6
          )
        ) {
          return <></>;
        }
        const other = getLogOther(record.other);
        const isSubscription = other?.billing_source === 'subscription';
        if (isSubscription) {
          // Subscription billed: show only tag (no $0), but keep tooltip for equivalent cost.
          return (
            <Tooltip content={`${t('\u7531\u8ba2\u9605\u62b5\u6263')}: ${renderQuota(text, 6)}`}>
              <span>{renderBillingTag(record, t)}</span>
            </Tooltip>
          );
        }
        return <>{renderQuota(text, 6)}</>;
      },
    },
    {
      key: COLUMN_KEYS.IP,
      title: (
        <div className='flex items-center gap-1'>
          {t('IP')}
          <Tooltip
            content={t(
              '只有当用户设置开启 IP 记录时，才会进行请求和错误类型日志的 IP 记录',
            )}
          >
            <IconHelpCircle className='text-gray-400 cursor-help' />
          </Tooltip>
        </div>
      ),
      dataIndex: 'ip',
      render: (text, record, index) => {
        const showIp =
          (record.type === 2 ||
            record.type === 5 ||
            (isAdminUser && record.type === 1)) &&
          text;
        return showIp ? (
          <Tooltip content={text}>
            <span>
              <Tag
                color='orange'
                shape='circle'
                onClick={(event) => {
                  copyText(event, text);
                }}
              >
                {text}
              </Tag>
            </span>
          </Tooltip>
        ) : (
          <></>
        );
      },
    },
    {
      key: COLUMN_KEYS.RETRY,
      title: t('\u91cd\u8bd5'),
      dataIndex: 'retry',
      render: (text, record, index) => {
        if (!(record.type === 2 || record.type === 5)) {
          return <></>;
        }
        let content = t('\u6e20\u9053') + ': ' + record.channel;
        if (record.other !== '') {
          let other = getLogOther(record.other);
          if (other === null) {
            return <></>;
          }
          if (other.admin_info !== undefined) {
            if (
              other.admin_info.use_channel !== null &&
              other.admin_info.use_channel !== undefined &&
              other.admin_info.use_channel !== ''
            ) {
              let useChannel = other.admin_info.use_channel;
              let useChannelStr = useChannel.join('->');
                content = t('\u6e20\u9053') + ': ' + useChannelStr;
            }
          }
        }
        return isAdminUser ? <div>{content}</div> : <></>;
      },
    },
    {
      key: COLUMN_KEYS.DETAILS,
      title: t('详情'),
      dataIndex: 'content',
      fixed: 'right',
      width: 200,
      render: (text, record, index) => {
        const detailSummary = getUsageLogDetailSummary(
          record,
          text,
          billingDisplayMode,
          t,
        );

        if (!detailSummary) {
          return (
            <Typography.Paragraph
              ellipsis={{
                rows: 2,
                showTooltip: {
                  type: 'popover',
                  opts: { style: { width: 240 } },
                },
              }}
              style={{ maxWidth: 200, marginBottom: 0 }}
            >
              {text}
            </Typography.Paragraph>
          );
        }

        const compactSummary = renderCompactDetailSummary(detailSummary.segments);
        const formulaContent = renderUsageLogFormulaContent(detailSummary.formula, t);

        if (!formulaContent) {
          return compactSummary;
        }

        return (
          <Popover
            trigger='click'
            position='leftTop'
            showArrow={true}
            content={formulaContent}
            style={{ maxWidth: '80vw' }}
          >
            <div style={{ cursor: 'pointer' }}>
              {compactSummary}
              <Typography.Text
                type='tertiary'
                size='small'
                style={{ display: 'block', marginTop: 2, fontSize: 11 }}
              >
                {t('点击查看公式')}
              </Typography.Text>
            </div>
          </Popover>
        );
      },
    },
  ];
};


