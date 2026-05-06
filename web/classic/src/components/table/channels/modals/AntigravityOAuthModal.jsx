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
import { Modal, Button, Space, Typography, Input, Banner } from '@douyinfe/semi-ui';
import { API, copy, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const AntigravityOAuthModal = ({ visible, onCancel, onSuccess }) => {
  const [loading, setLoading] = useState(false);
  const [authorizeUrl, setAuthorizeUrl] = useState('');
  const [input, setInput] = useState('');

  const ensureAuthorizeUrl = async () => {
    if (authorizeUrl) {
      return authorizeUrl;
    }

    const res = await API.post(
      '/api/channel/antigravity/oauth/start',
      {},
      { skipErrorHandler: true },
    );
    if (!res?.data?.success) {
      throw new Error(res?.data?.message || 'Failed to start authorization');
    }

    const url = res?.data?.data?.authorize_url || '';
    if (!url) {
      throw new Error('Authorization link is missing in the response');
    }

    setAuthorizeUrl(url);
    return url;
  };

  const startOAuth = async () => {
    setLoading(true);
    try {
      const url = await ensureAuthorizeUrl();
      window.open(url, '_blank', 'noopener,noreferrer');
      showSuccess('Authorization page opened');
    } catch (error) {
      showError(error?.message || 'Failed to start authorization');
    } finally {
      setLoading(false);
    }
  };

  const copyAuthorizeUrl = async () => {
    setLoading(true);
    try {
      const url = await ensureAuthorizeUrl();
      await copy(url);
      showSuccess('Authorization link copied');
    } catch (error) {
      showError(error?.message || 'Failed to copy authorization link');
    } finally {
      setLoading(false);
    }
  };

  const completeOAuth = async () => {
    if (!input || !input.trim()) {
      showError('Please paste the callback URL first');
      return;
    }

    setLoading(true);
    try {
      const res = await API.post(
        '/api/channel/antigravity/oauth/complete',
        { input },
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || 'Authorization failed');
      }

      const key = res?.data?.data?.key || '';
      if (!key) {
        throw new Error('Credential is missing in the response');
      }

      onSuccess && onSuccess(key);
      showSuccess('Authorization credential generated');
      onCancel && onCancel();
    } catch (error) {
      showError(error?.message || 'Authorization failed');
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
      title='Antigravity OAuth'
      visible={visible}
      onCancel={onCancel}
      maskClosable={false}
      closeOnEsc
      width={720}
      footer={
        <Space>
          <Button theme='borderless' onClick={onCancel} disabled={loading}>
            Cancel
          </Button>
          <Button
            theme='solid'
            type='primary'
            onClick={completeOAuth}
            loading={loading}
          >
            Generate And Fill
          </Button>
        </Space>
      }
    >
      <Space vertical spacing='tight' style={{ width: '100%' }}>
        <Banner
          type='info'
          description='1) Open the authorization page and complete Google sign-in. 2) The browser may redirect to a localhost page; it is fine if that page does not open correctly. 3) Copy the full callback URL from the address bar and paste it below. 4) Click "Generate And Fill".'
        />

        <Space wrap>
          <Button type='primary' onClick={startOAuth} loading={loading}>
            Open Authorization Page
          </Button>
          <Button theme='outline' disabled={loading} onClick={copyAuthorizeUrl}>
            Copy Authorization Link
          </Button>
        </Space>

        <Input
          value={input}
          onChange={(value) => setInput(value)}
          placeholder='Paste the full callback URL here, including code and state'
          showClear
        />

        <Text type='tertiary' size='small'>
          The generated result is a JSON credential that can be pasted directly into the channel key field, including access token, refresh token, email, and project_id when returned by upstream.
        </Text>
      </Space>
    </Modal>
  );
};

export default AntigravityOAuthModal;
