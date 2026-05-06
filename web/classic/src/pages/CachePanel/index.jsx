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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Descriptions,
  Progress,
  Row,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, isAdmin, showError, timestamp2string } from '../../helpers';

const REFRESH_INTERVAL_MS = 10_000;

function toPercent(value) {
  const n = Number(value || 0);
  if (!Number.isFinite(n) || n <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(100, n * 100));
}

const CachePanel = () => {
  const { t } = useTranslation();
  const [windowSeconds, setWindowSeconds] = useState(600);
  const [scope, setScope] = useState('all');
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState(null);
  const canSelectScope = isAdmin();

  const scopeOptions = useMemo(() => {
    const options = [{ label: t('全站'), value: 'all' }];
    if (canSelectScope) {
      options.push({ label: t('仅自己'), value: 'self' });
    }
    return options;
  }, [canSelectScope, t]);

  const windowOptions = useMemo(
    () => [
      { label: t('5 分钟'), value: 300 },
      { label: t('10 分钟'), value: 600 },
      { label: t('30 分钟'), value: 1800 },
      { label: t('1 小时'), value: 3600 },
      { label: t('6 小时'), value: 21600 },
      { label: t('24 小时'), value: 86400 },
    ],
    [t],
  );

  const loadStats = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/log/cache_panel', {
        params: {
          window_seconds: windowSeconds,
          scope: canSelectScope ? scope : 'self',
          limit: 2000,
        },
        disableDuplicate: true,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('获取统计失败'));
        return;
      }
      setStats(data || null);
    } catch (error) {
      showError(t('获取统计失败'));
    } finally {
      setLoading(false);
    }
  }, [canSelectScope, scope, t, windowSeconds]);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  useEffect(() => {
    const timer = setInterval(() => {
      loadStats();
    }, REFRESH_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [loadStats]);

  const readTokenPercent = toPercent(stats?.cache_read_token_ratio);
  const writeTokenPercent = toPercent(stats?.cache_write_token_ratio);
  const readReqPercent = toPercent(stats?.cache_read_request_ratio);
  const writeReqPercent = toPercent(stats?.cache_write_request_ratio);

  const summaryRows = [
    { key: t('样本请求数'), value: stats?.sample_size || 0 },
    { key: t('总请求数'), value: stats?.total_requests || 0 },
    { key: t('缓存读请求数'), value: stats?.cache_read_requests || 0 },
    { key: t('缓存写请求数'), value: stats?.cache_write_requests || 0 },
    { key: t('仅缓存读请求数'), value: stats?.cache_read_only_requests || 0 },
    { key: t('仅缓存写请求数'), value: stats?.cache_write_only_requests || 0 },
    { key: t('读写混合请求数'), value: stats?.cache_mixed_requests || 0 },
    { key: t('无缓存请求数'), value: stats?.cache_none_requests || 0 },
    { key: t('缓存读 Tokens'), value: stats?.cache_read_tokens || 0 },
    { key: t('缓存写 Tokens'), value: stats?.cache_write_tokens || 0 },
    { key: t('输出 Tokens'), value: stats?.completion_tokens || 0 },
    {
      key: t('最近更新时间'),
      value: stats?.updated_at ? timestamp2string(stats.updated_at) : '-',
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Card
        title={t('缓存面板')}
        extra={
          <Space>
            <Select
              style={{ width: 120 }}
              value={windowSeconds}
              optionList={windowOptions}
              onChange={(value) => setWindowSeconds(value)}
            />
            <Select
              style={{ width: 100 }}
              value={canSelectScope ? scope : 'self'}
              optionList={scopeOptions}
              disabled={!canSelectScope}
              onChange={(value) => setScope(value)}
            />
            <Button onClick={loadStats}>{t('刷新')}</Button>
          </Space>
        }
      >
        <Spin spinning={loading}>
          <Row gutter={16}>
            <Col span={12}>
              <Card>
                <Typography.Text strong>{t('缓存 Tokens 占比')}</Typography.Text>
                <div style={{ marginTop: 12 }}>
                  <Typography.Text>
                    {t('读占比')} {readTokenPercent.toFixed(2)}%
                  </Typography.Text>
                  <Progress percent={readTokenPercent} stroke='#0ea5e9' />
                </div>
                <div style={{ marginTop: 12 }}>
                  <Typography.Text>
                    {t('写占比')} {writeTokenPercent.toFixed(2)}%
                  </Typography.Text>
                  <Progress percent={writeTokenPercent} stroke='#22c55e' />
                </div>
              </Card>
            </Col>
            <Col span={12}>
              <Card>
                <Typography.Text strong>{t('缓存请求占比')}</Typography.Text>
                <div style={{ marginTop: 12 }}>
                  <Typography.Text>
                    {t('读占比')} {readReqPercent.toFixed(2)}%
                  </Typography.Text>
                  <Progress percent={readReqPercent} stroke='#0284c7' />
                </div>
                <div style={{ marginTop: 12 }}>
                  <Typography.Text>
                    {t('写占比')} {writeReqPercent.toFixed(2)}%
                  </Typography.Text>
                  <Progress percent={writeReqPercent} stroke='#16a34a' />
                </div>
                <div style={{ marginTop: 12 }}>
                  <Tag color='blue'>{t('每 10 秒自动刷新')}</Tag>
                </div>
              </Card>
            </Col>
          </Row>

          <div style={{ marginTop: 16 }}>
            <Card>
              <Descriptions data={summaryRows} />
            </Card>
          </div>
        </Spin>
      </Card>
    </div>
  );
};

export default CachePanel;
