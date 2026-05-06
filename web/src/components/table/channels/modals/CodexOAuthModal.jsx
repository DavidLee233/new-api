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

import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Button, Space, Typography, Input, Banner } from '@douyinfe/semi-ui';
import { API, copy, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const CodexOAuthModal = ({
  visible,
  onCancel,
  onSuccess,
  channelId,
  mode = 'fill',
}) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [authorizeUrl, setAuthorizeUrl] = useState('');
  const [input, setInput] = useState('');
  const isAppendMode = mode === 'append' && Number(channelId) > 0;

  const getStartPath = () => {
    if (isAppendMode) {
      return `/api/channel/${channelId}/codex/oauth/start`;
    }
    return '/api/channel/codex/oauth/start';
  };

  const getCompletePath = () => {
    if (isAppendMode) {
      return `/api/channel/${channelId}/codex/oauth/complete?append=1`;
    }
    return '/api/channel/codex/oauth/complete';
  };

  const ensureAuthorizeUrl = async () => {
    if (authorizeUrl) {
      return authorizeUrl;
    }

    const res = await API.post(
      getStartPath(),
      {},
      { skipErrorHandler: true },
    );
    if (!res?.data?.success) {
      console.error('Codex OAuth start failed:', res?.data?.message);
      throw new Error(t('启动授权失败'));
    }

    const url = res?.data?.data?.authorize_url || '';
    if (!url) {
      console.error('Codex OAuth start response missing authorize_url:', res?.data);
      throw new Error(t('响应缺少授权链接'));
    }

    setAuthorizeUrl(url);
    return url;
  };

  const startOAuth = async () => {
    setLoading(true);
    try {
      const url = await ensureAuthorizeUrl();
      window.open(url, '_blank', 'noopener,noreferrer');
      showSuccess(t('已打开授权页面'));
    } catch (error) {
      showError(error?.message || t('启动授权失败'));
    } finally {
      setLoading(false);
    }
  };

  const copyAuthorizeUrl = async () => {
    setLoading(true);
    try {
      const url = await ensureAuthorizeUrl();
      await copy(url);
      showSuccess(t('授权链接已复制'));
    } catch (error) {
      showError(error?.message || t('复制授权链接失败'));
    } finally {
      setLoading(false);
    }
  };

  const completeOAuth = async () => {
    if (!input || !input.trim()) {
      showError(t('请先粘贴回调 URL'));
      return;
    }

    setLoading(true);
    try {
      const res = await API.post(
        getCompletePath(),
        { input, append: isAppendMode },
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        console.error('Codex OAuth complete failed:', res?.data?.message);
        throw new Error(t('授权失败'));
      }

      if (isAppendMode) {
        onSuccess && onSuccess(res?.data?.data || {});
        showSuccess(t('账号已追加到渠道 key 列表'));
        onCancel && onCancel();
        return;
      }

      const key = res?.data?.data?.key || '';
      if (!key) {
        console.error('Codex OAuth complete response missing key:', res?.data);
        throw new Error(t('响应缺少凭据'));
      }
      onSuccess && onSuccess(key);
      showSuccess(t('已生成授权凭据'));
      onCancel && onCancel();
    } catch (error) {
      showError(error?.message || t('授权失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) return;
    setAuthorizeUrl('');
    setInput('');
  }, [visible]);

  return (
    <Modal
      title={t('Codex 授权')}
      visible={visible}
      onCancel={onCancel}
      maskClosable={false}
      closeOnEsc
      width={720}
      footer={
        <Space>
          <Button theme='borderless' onClick={onCancel} disabled={loading}>
            {t('取消')}
          </Button>
          <Button
            theme='solid'
            type='primary'
            onClick={completeOAuth}
            loading={loading}
          >
            {isAppendMode ? t('追加到渠道') : t('生成并填入')}
          </Button>
        </Space>
      }
    >
      <Space vertical spacing='tight' style={{ width: '100%' }}>
        <Banner
          type='info'
          description={
            isAppendMode
              ? t(
                  '1) 点击“打开授权页面”完成登录；2) 浏览器可能会跳转到 localhost 页面，打不开也没关系；3) 复制地址栏里的完整回调 URL 粘贴到下方；4) 点击“追加到渠道”。',
                )
              : t(
                  '1) 点击“打开授权页面”完成登录；2) 浏览器可能会跳转到 localhost 页面，打不开也没关系；3) 复制地址栏里的完整回调 URL 粘贴到下方；4) 点击“生成并填入”。',
                )
          }
        />

        <Space wrap>
          <Button type='primary' onClick={startOAuth} loading={loading}>
            {t('打开授权页面')}
          </Button>
          <Button theme='outline' disabled={loading} onClick={copyAuthorizeUrl}>
            {t('复制授权链接')}
          </Button>
        </Space>

        <Input
          value={input}
          onChange={(value) => setInput(value)}
          placeholder={t('请粘贴完整回调 URL（包含 code 与 state）')}
          showClear
        />

        <Text type='tertiary' size='small'>
          {t(
            '生成结果是可直接粘贴到渠道密钥里的 JSON 凭据，包含 access_token、refresh_token 和 account_id。',
          )}
        </Text>
      </Space>
    </Modal>
  );
};

export default CodexOAuthModal;
