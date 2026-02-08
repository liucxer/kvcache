// 页面加载完成后执行
window.addEventListener('DOMContentLoaded', function() {
    // 初始化页面
    initTabs();
    initHealthCheck();
    initSingleOps();
    initBatchOps();
    initScanOps();
    initConfig();
    initMetrics();
});

// 初始化标签页
function initTabs() {
    const navLinks = document.querySelectorAll('.nav-link');
    const tabContents = document.querySelectorAll('.tab-content');

    navLinks.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();

            // 移除所有活动状态
            navLinks.forEach(l => l.classList.remove('active'));
            tabContents.forEach(c => c.classList.add('hidden'));

            // 设置当前活动状态
            this.classList.add('active');
            const tabId = this.getAttribute('data-tab');
            document.getElementById(tabId).classList.remove('hidden');
        });
    });
}

// 初始化健康检查
function initHealthCheck() {
    const healthCheckBtn = document.getElementById('health-check-btn');
    const healthStatus = document.getElementById('health-status');
    const healthMessage = document.getElementById('health-message');

    // 初始检查
    checkHealth();

    // 绑定检查按钮
    healthCheckBtn.addEventListener('click', checkHealth);

    // 检查健康状态
    function checkHealth() {
        fetch('/health')
            .then(response => response.json())
            .then(data => {
                if (data.status === 'healthy') {
                    healthStatus.textContent = '健康';
                    healthStatus.className = 'text-lg font-medium text-green-600';
                    healthMessage.textContent = data.message;
                    healthMessage.className = 'alert alert-success';
                } else {
                    healthStatus.textContent = '不健康';
                    healthStatus.className = 'text-lg font-medium text-red-600';
                    healthMessage.textContent = data.message;
                    healthMessage.className = 'alert alert-danger';
                }
                healthMessage.classList.remove('hidden');
            })
            .catch(error => {
                healthStatus.textContent = '错误';
                healthStatus.className = 'text-lg font-medium text-red-600';
                healthMessage.textContent = '无法连接到服务: ' + error.message;
                healthMessage.className = 'alert alert-danger';
                healthMessage.classList.remove('hidden');
            });
    }
}

// 初始化单个键值操作
function initSingleOps() {
    // 设置键值对
    const setForm = document.getElementById('set-form');
    setForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const key = document.getElementById('set-key').value;
        const value = document.getElementById('set-value').value;
        const ttl = document.getElementById('set-ttl').value;

        const data = {
            key: key,
            value: value
        };

        if (ttl) {
            data.ttl = parseInt(ttl);
        }

        fetch('/api/v1/set', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(data)
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showAlert('success', '设置成功: ' + data.message);
                setForm.reset();
            } else {
                showAlert('danger', '设置失败: ' + data.error);
            }
        })
        .catch(error => {
            showAlert('danger', '请求失败: ' + error.message);
        });
    });

    // 获取值
    const getForm = document.getElementById('get-form');
    const getResult = document.getElementById('get-result');

    getForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const key = document.getElementById('get-key').value;

        fetch(`/api/v1/get/${key}`)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                getResult.innerHTML = `<div class="alert alert-danger">${data.error}</div>`;
            } else {
                getResult.innerHTML = `
                    <div class="bg-gray-50 p-4 rounded-md">
                        <p class="font-medium">键: ${key}</p>
                        <p class="font-medium">值:</p>
                        <pre class="bg-white p-3 rounded-md border border-gray-200 mt-2">${data.value}</pre>
                    </div>
                `;
            }
            getResult.classList.remove('hidden');
        })
        .catch(error => {
            getResult.innerHTML = `<div class="alert alert-danger">请求失败: ${error.message}</div>`;
            getResult.classList.remove('hidden');
        });
    });

    // 删除键值对
    const deleteForm = document.getElementById('delete-form');

    deleteForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const key = document.getElementById('delete-key').value;

        fetch(`/api/v1/delete/${key}`, {
            method: 'DELETE'
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showAlert('success', '删除成功: ' + data.message);
                deleteForm.reset();
            } else {
                showAlert('danger', '删除失败: ' + data.error);
            }
        })
        .catch(error => {
            showAlert('danger', '请求失败: ' + error.message);
        });
    });
}

// 初始化批量键值操作
function initBatchOps() {
    // 批量设置键值对
    const msetForm = document.getElementById('mset-form');

    msetForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const kvs = {};
        const kvInputs = document.querySelectorAll('#mset-kvs input[type="text"]');

        for (let i = 0; i < kvInputs.length; i += 2) {
            const key = kvInputs[i].value;
            const value = kvInputs[i + 1].value;
            if (key && value) {
                kvs[key] = value;
            }
        }

        const ttl = document.getElementById('mset-ttl').value;

        const data = {
            kvs: kvs
        };

        if (ttl) {
            data.ttl = parseInt(ttl);
        }

        fetch('/api/v1/mset', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(data)
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showAlert('success', '批量设置成功: ' + data.message);
                msetForm.reset();
                document.getElementById('mset-kvs').innerHTML = `
                    <div class="flex space-x-2">
                        <input type="text" class="form-input" placeholder="键" required>
                        <input type="text" class="form-input" placeholder="值" required>
                        <button type="button" class="btn btn-outline flex-shrink-0" onclick="removeKV(this)">
                            <i class="fa fa-times"></i>
                        </button>
                    </div>
                `;
            } else {
                showAlert('danger', '批量设置失败: ' + data.error);
            }
        })
        .catch(error => {
            showAlert('danger', '请求失败: ' + error.message);
        });
    });

    // 批量获取值
    const mgetForm = document.getElementById('mget-form');
    const mgetResult = document.getElementById('mget-result');

    mgetForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const keys = [];
        const keyInputs = document.querySelectorAll('#mget-keys input[type="text"]');

        keyInputs.forEach(input => {
            if (input.value) {
                keys.push(input.value);
            }
        });

        fetch('/api/v1/mget', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ keys: keys })
        })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                mgetResult.innerHTML = `<div class="alert alert-danger">${data.error}</div>`;
            } else {
                let resultHtml = `
                    <div class="bg-gray-50 p-4 rounded-md">
                        <p class="font-medium mb-2">获取结果 (${data.count} 个键):</p>
                        <div class="space-y-2">
                `;

                for (const [key, value] of Object.entries(data.results)) {
                    resultHtml += `
                        <div class="bg-white p-3 rounded-md border border-gray-200">
                            <p class="font-medium">键: ${key}</p>
                            <pre class="mt-1">${value}</pre>
                        </div>
                    `;
                }

                resultHtml += `
                        </div>
                    </div>
                `;

                mgetResult.innerHTML = resultHtml;
            }
            mgetResult.classList.remove('hidden');
        })
        .catch(error => {
            mgetResult.innerHTML = `<div class="alert alert-danger">请求失败: ${error.message}</div>`;
            mgetResult.classList.remove('hidden');
        });
    });

    // 批量删除键值对
    const mdeleteForm = document.getElementById('mdelete-form');

    mdeleteForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const keys = [];
        const keyInputs = document.querySelectorAll('#mdelete-keys input[type="text"]');

        keyInputs.forEach(input => {
            if (input.value) {
                keys.push(input.value);
            }
        });

        fetch('/api/v1/mdelete', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ keys: keys })
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showAlert('success', '批量删除成功: ' + data.message);
                mdeleteForm.reset();
                document.getElementById('mdelete-keys').innerHTML = `
                    <div class="flex space-x-2">
                        <input type="text" class="form-input" placeholder="键" required>
                        <button type="button" class="btn btn-outline flex-shrink-0" onclick="removeKey(this)">
                            <i class="fa fa-times"></i>
                        </button>
                    </div>
                `;
            } else {
                showAlert('danger', '批量删除失败: ' + data.error);
            }
        })
        .catch(error => {
            showAlert('danger', '请求失败: ' + error.message);
        });
    });
}

// 初始化扫描操作
function initScanOps() {
    const scanForm = document.getElementById('scan-form');
    const scanResult = document.getElementById('scan-result');
    const scanKeysBtn = document.getElementById('scan-keys-btn');

    // 扫描键值对
    scanForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const prefix = document.getElementById('scan-prefix').value;

        fetch(`/api/v1/scan?prefix=${encodeURIComponent(prefix)}`)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                scanResult.innerHTML = `<div class="alert alert-danger">${data.error}</div>`;
            } else {
                let resultHtml = `
                    <div class="bg-gray-50 p-4 rounded-md">
                        <p class="font-medium mb-2">扫描结果 (${data.count} 个键值对):</p>
                        <div class="overflow-x-auto">
                            <table class="min-w-full bg-white border border-gray-200 rounded-md">
                                <thead>
                                    <tr class="bg-gray-100">
                                        <th class="py-2 px-4 border-b">键</th>
                                        <th class="py-2 px-4 border-b">值</th>
                                    </tr>
                                </thead>
                                <tbody>
                `;

                for (const [key, value] of Object.entries(data.results)) {
                    resultHtml += `
                        <tr>
                            <td class="py-2 px-4 border-b">${key}</td>
                            <td class="py-2 px-4 border-b"><pre class="whitespace-pre-wrap">${value}</pre></td>
                        </tr>
                    `;
                }

                resultHtml += `
                                </tbody>
                            </table>
                        </div>
                    </div>
                `;

                scanResult.innerHTML = resultHtml;
            }
            scanResult.classList.remove('hidden');
        })
        .catch(error => {
            scanResult.innerHTML = `<div class="alert alert-danger">请求失败: ${error.message}</div>`;
            scanResult.classList.remove('hidden');
        });
    });

    // 只扫描键
    scanKeysBtn.addEventListener('click', function() {
        const prefix = document.getElementById('scan-prefix').value;

        fetch(`/api/v1/scan?prefix=${encodeURIComponent(prefix)}`)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                scanResult.innerHTML = `<div class="alert alert-danger">${data.error}</div>`;
            } else {
                const keys = Object.keys(data.results);
                let resultHtml = `
                    <div class="bg-gray-50 p-4 rounded-md">
                        <p class="font-medium mb-2">扫描结果 (${keys.length} 个键):</p>
                        <div class="bg-white p-3 rounded-md border border-gray-200">
                            <ul class="space-y-1">
                `;

                keys.forEach(key => {
                    resultHtml += `<li><i class="fa fa-key mr-2 text-primary"></i>${key}</li>`;
                });

                resultHtml += `
                            </ul>
                        </div>
                    </div>
                `;

                scanResult.innerHTML = resultHtml;
            }
            scanResult.classList.remove('hidden');
        })
        .catch(error => {
            scanResult.innerHTML = `<div class="alert alert-danger">请求失败: ${error.message}</div>`;
            scanResult.classList.remove('hidden');
        });
    });
}

// 初始化配置管理
function initConfig() {
    // 获取当前配置
    fetch('/api/v1/config')
    .then(response => response.json())
    .then(data => {
        const currentConfig = document.getElementById('current-config');
        currentConfig.innerHTML = `
            <pre class="whitespace-pre-wrap">${JSON.stringify(data, null, 2)}</pre>
        `;
    })
    .catch(error => {
        const currentConfig = document.getElementById('current-config');
        currentConfig.innerHTML = `<div class="alert alert-danger">获取配置失败: ${error.message}</div>`;
    });

    // 更新配置
    const updateConfigForm = document.getElementById('update-config-form');
    const configMessage = document.getElementById('config-message');

    updateConfigForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const rocksdbPath = document.getElementById('rocksdb-path').value;
        const diskStorePath = document.getElementById('disk-store-path').value;
        const largeValueSize = document.getElementById('large-value-size').value;
        const maxDiskUsage = document.getElementById('max-disk-usage').value;
        const evictionCheckInterval = document.getElementById('eviction-check-interval').value;
        const evictionBatchSize = document.getElementById('eviction-batch-size').value;

        const data = {};

        if (rocksdbPath) {
            data.rocksdb_path = rocksdbPath;
        }
        if (diskStorePath) {
            data.disk_store_path = diskStorePath;
        }
        if (largeValueSize) {
            data.large_value_size = parseInt(largeValueSize);
        }
        if (maxDiskUsage) {
            data.max_disk_usage = parseFloat(maxDiskUsage);
        }
        if (evictionCheckInterval) {
            data.eviction_check_interval = parseInt(evictionCheckInterval);
        }
        if (evictionBatchSize) {
            data.eviction_batch_size = parseInt(evictionBatchSize);
        }

        fetch('/api/v1/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(data)
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                configMessage.className = 'alert alert-success';
                configMessage.textContent = '配置更新成功: ' + data.message;
                configMessage.classList.remove('hidden');
                updateConfigForm.reset();
                // 重新获取配置
                fetch('/api/v1/config')
                .then(response => response.json())
                .then(data => {
                    const currentConfig = document.getElementById('current-config');
                    currentConfig.innerHTML = `
                        <pre class="whitespace-pre-wrap">${JSON.stringify(data, null, 2)}</pre>
                    `;
                });
            } else {
                configMessage.className = 'alert alert-danger';
                configMessage.textContent = '配置更新失败: ' + data.error;
                configMessage.classList.remove('hidden');
            }
        })
        .catch(error => {
            configMessage.className = 'alert alert-danger';
            configMessage.textContent = '请求失败: ' + error.message;
            configMessage.classList.remove('hidden');
        });
    });
}

// 初始化监控指标
function initMetrics() {
    // 模拟监控指标
    document.getElementById('metric-ops').textContent = '1,234';
    document.getElementById('metric-errors').textContent = '5';
    document.getElementById('metric-keys').textContent = '567';
}

// 添加键值对
function addKV(formId) {
    const container = document.getElementById(`${formId}-kvs`);
    const newKV = document.createElement('div');
    newKV.className = 'flex space-x-2';
    newKV.innerHTML = `
        <input type="text" class="form-input" placeholder="键" required>
        <input type="text" class="form-input" placeholder="值" required>
        <button type="button" class="btn btn-outline flex-shrink-0" onclick="removeKV(this)">
            <i class="fa fa-times"></i>
        </button>
    `;
    container.appendChild(newKV);
}

// 移除键值对
function removeKV(button) {
    const kvDiv = button.parentElement;
    kvDiv.remove();
}

// 添加键
function addKey(formId) {
    const container = document.getElementById(`${formId}-keys`);
    const newKey = document.createElement('div');
    newKey.className = 'flex space-x-2';
    newKey.innerHTML = `
        <input type="text" class="form-input" placeholder="键" required>
        <button type="button" class="btn btn-outline flex-shrink-0" onclick="removeKey(this)">
            <i class="fa fa-times"></i>
        </button>
    `;
    container.appendChild(newKey);
}

// 移除键
function removeKey(button) {
    const keyDiv = button.parentElement;
    keyDiv.remove();
}

// 显示警报
function showAlert(type, message) {
    const resultDiv = document.getElementById('result');
    resultDiv.className = `fixed top-4 right-4 max-w-md z-10 alert alert-${type}`;
    resultDiv.textContent = message;
    resultDiv.classList.remove('hidden');

    // 3秒后自动隐藏
    setTimeout(function() {
        resultDiv.classList.add('hidden');
    }, 3000);
}

// 显示加载状态
function showLoading(element) {
    const originalText = element.textContent;
    element.disabled = true;
    element.innerHTML = '<i class="fa fa-spinner fa-spin mr-2"></i>加载中...';
    return originalText;
}

// 隐藏加载状态
function hideLoading(element, originalText) {
    element.disabled = false;
    element.innerHTML = originalText;
}
