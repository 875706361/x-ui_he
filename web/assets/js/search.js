// 全局搜索功能
class SearchManager {
    constructor() {
        this.searchResults = [];
        this.searchQuery = '';
        this.searchVisible = false;
        this.isSearching = false;
        this.init();
    }

    init() {
        // 创建搜索按钮和搜索框
        this.createSearchButton();
        this.createSearchModal();
        
        // 添加全局快捷键
        document.addEventListener('keydown', (e) => {
            // Ctrl+F 或 Command+F 打开搜索框
            if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
                e.preventDefault();
                this.showSearch();
            }
            
            // ESC 关闭搜索框
            if (e.key === 'Escape' && this.searchVisible) {
                this.hideSearch();
            }
        });
    }

    // 创建搜索按钮
    createSearchButton() {
        const button = document.createElement('div');
        button.id = 'search-button';
        button.className = 'search-button';
        button.title = '搜索';
        button.innerHTML = `
            <i class="anticon">
                <svg viewBox="64 64 896 896" focusable="false" width="1em" height="1em" fill="currentColor" aria-hidden="true">
                    <path d="M909.6 854.5L649.9 594.8C690.2 542.7 712 479 712 412c0-80.2-31.3-155.4-87.9-212.1-56.6-56.7-132-87.9-212.1-87.9s-155.5 31.3-212.1 87.9C143.2 256.5 112 331.8 112 412c0 80.1 31.3 155.5 87.9 212.1C256.5 680.8 331.8 712 412 712c67 0 130.6-21.8 182.7-62l259.7 259.6a8.2 8.2 0 0011.6 0l43.6-43.5a8.2 8.2 0 000-11.6zM570.4 570.4C528 612.7 471.8 636 412 636s-116-23.3-158.4-65.6C211.3 528 188 471.8 188 412s23.3-116.1 65.6-158.4C296 211.3 352.2 188 412 188s116.1 23.2 158.4 65.6S636 352.2 636 412s-23.3 116.1-65.6 158.4z"></path>
                </svg>
            </i>
        `;
        
        button.addEventListener('click', () => {
            this.showSearch();
        });
        
        document.body.appendChild(button);
    }

    // 创建搜索模态框
    createSearchModal() {
        const modal = document.createElement('div');
        modal.id = 'search-modal';
        modal.className = 'search-modal';
        modal.style.display = 'none';
        
        modal.innerHTML = `
            <div class="search-modal-content">
                <div class="search-header">
                    <div class="search-input-container">
                        <input type="text" id="search-input" placeholder="输入关键词搜索..." />
                        <div class="search-loading" id="search-loading" style="display: none;">
                            <i class="anticon anticon-loading anticon-spin">
                                <svg viewBox="0 0 1024 1024" focusable="false" width="1em" height="1em" fill="currentColor" aria-hidden="true">
                                    <path d="M988 548c-19.9 0-36-16.1-36-36 0-59.4-11.6-117-34.6-171.3a440.45 440.45 0 00-94.3-139.9 437.71 437.71 0 00-139.9-94.3C629 83.6 571.4 72 512 72c-19.9 0-36-16.1-36-36s16.1-36 36-36c69.1 0 136.2 13.5 199.3 40.3C772.3 66 827 103 874 150c47 47 83.9 101.8 109.7 162.7 26.7 63.1 40.2 130.2 40.2 199.3.1 19.9-16 36-35.9 36z"></path>
                                </svg>
                            </i>
                        </div>
                    </div>
                    <div class="search-close" id="search-close">
                        <i class="anticon">
                            <svg viewBox="64 64 896 896" focusable="false" width="1em" height="1em" fill="currentColor" aria-hidden="true">
                                <path d="M563.8 512l262.5-312.9c4.4-5.2.7-13.1-6.1-13.1h-79.8c-4.7 0-9.2 2.1-12.3 5.7L511.6 449.8 295.1 191.7c-3-3.6-7.5-5.7-12.3-5.7H203c-6.8 0-10.5 7.9-6.1 13.1L459.4 512 196.9 824.9A7.95 7.95 0 00203 838h79.8c4.7 0 9.2-2.1 12.3-5.7l216.5-258.1 216.5 258.1c3 3.6 7.5 5.7 12.3 5.7h79.8c6.8 0 10.5-7.9 6.1-13.1L563.8 512z"></path>
                            </svg>
                        </i>
                    </div>
                </div>
                <div class="search-results" id="search-results"></div>
            </div>
        `;
        
        document.body.appendChild(modal);
        
        // 添加事件监听
        const searchInput = document.getElementById('search-input');
        const searchClose = document.getElementById('search-close');
        
        searchInput.addEventListener('input', () => {
            this.searchQuery = searchInput.value;
            if (this.searchQuery.length >= 2) {
                this.performSearch();
            } else {
                this.clearResults();
            }
        });
        
        searchClose.addEventListener('click', () => {
            this.hideSearch();
        });
    }

    // 显示搜索框
    showSearch() {
        const modal = document.getElementById('search-modal');
        const searchInput = document.getElementById('search-input');
        
        if (modal) {
            modal.style.display = 'flex';
            this.searchVisible = true;
            
            // 聚焦输入框
            setTimeout(() => {
                searchInput.focus();
            }, 100);
        }
    }

    // 隐藏搜索框
    hideSearch() {
        const modal = document.getElementById('search-modal');
        const searchInput = document.getElementById('search-input');
        
        if (modal) {
            modal.style.display = 'none';
            this.searchVisible = false;
            searchInput.value = '';
            this.searchQuery = '';
            this.clearResults();
        }
    }

    // 执行搜索
    async performSearch() {
        if (this.isSearching || !this.searchQuery) return;
        
        this.isSearching = true;
        document.getElementById('search-loading').style.display = 'block';
        
        try {
            // 搜索入站和客户端
            const inbounds = await this.searchInbounds(this.searchQuery);
            
            // 显示结果
            this.displayResults(inbounds);
        } catch (error) {
            console.error('搜索出错:', error);
        } finally {
            this.isSearching = false;
            document.getElementById('search-loading').style.display = 'none';
        }
    }

    // 搜索入站和客户端
    async searchInbounds(query) {
        // 获取所有入站
        const response = await HttpUtil.post('/xray/inbounds');
        if (!response.success) {
            return [];
        }
        
        const inbounds = response.obj;
        const results = [];
        
        // 搜索匹配的入站
        const lowercaseQuery = query.toLowerCase();
        
        for (const inbound of inbounds) {
            const inboundMatches = 
                (inbound.remark && inbound.remark.toLowerCase().includes(lowercaseQuery)) || 
                (inbound.protocol && inbound.protocol.toLowerCase().includes(lowercaseQuery)) ||
                (inbound.port && inbound.port.toString().includes(lowercaseQuery));
            
            // 如果入站匹配，添加到结果中
            if (inboundMatches) {
                results.push({
                    type: 'inbound',
                    id: inbound.id,
                    remark: inbound.remark || '未命名',
                    protocol: inbound.protocol,
                    port: inbound.port,
                    highlight: this.highlightMatch(inbound.remark || '未命名', lowercaseQuery)
                });
            }
            
            // 搜索客户端
            if (inbound.settings) {
                try {
                    const settings = JSON.parse(inbound.settings);
                    
                    if (settings.clients && Array.isArray(settings.clients)) {
                        for (const client of settings.clients) {
                            const clientMatches = 
                                (client.email && client.email.toLowerCase().includes(lowercaseQuery)) ||
                                (client.id && client.id.toLowerCase().includes(lowercaseQuery));
                            
                            if (clientMatches) {
                                results.push({
                                    type: 'client',
                                    inboundId: inbound.id,
                                    inboundRemark: inbound.remark || '未命名',
                                    email: client.email || '未命名',
                                    id: client.id,
                                    highlight: this.highlightMatch(client.email || client.id, lowercaseQuery)
                                });
                            }
                        }
                    }
                } catch (e) {
                    console.error('解析客户端设置失败:', e);
                }
            }
        }
        
        return results;
    }

    // 高亮匹配文本
    highlightMatch(text, query) {
        if (!text) return '';
        
        const regex = new RegExp(`(${query})`, 'gi');
        return text.replace(regex, '<span class="search-highlight">$1</span>');
    }

    // 显示搜索结果
    displayResults(results) {
        const resultsContainer = document.getElementById('search-results');
        resultsContainer.innerHTML = '';
        
        if (results.length === 0) {
            resultsContainer.innerHTML = '<div class="search-no-results">没有找到匹配的结果</div>';
            return;
        }
        
        // 创建结果列表
        const resultsList = document.createElement('div');
        resultsList.className = 'search-results-list';
        
        results.forEach(result => {
            const resultItem = document.createElement('div');
            resultItem.className = 'search-result-item';
            
            if (result.type === 'inbound') {
                resultItem.innerHTML = `
                    <div class="search-result-icon">
                        <i class="anticon">
                            <svg viewBox="64 64 896 896" focusable="false" width="1em" height="1em" fill="currentColor" aria-hidden="true">
                                <path d="M880 112H144c-17.7 0-32 14.3-32 32v736c0 17.7 14.3 32 32 32h736c17.7 0 32-14.3 32-32V144c0-17.7-14.3-32-32-32zM368 744c0 4.4-3.6 8-8 8h-80c-4.4 0-8-3.6-8-8V280c0-4.4 3.6-8 8-8h80c4.4 0 8 3.6 8 8v464zm192-280c0 4.4-3.6 8-8 8h-80c-4.4 0 8 3.6 8 8v280c0 4.4-3.6 8-8 8h-80c-4.4 0-8-3.6-8-8V472c0-4.4 3.6-8 8-8h80c4.4 0 8 3.6 8 8v280c0 4.4-3.6 8-8 8h-80c-4.4 0-8-3.6-8-8V280c0-4.4 3.6-8 8-8h80c4.4 0 8 3.6 8 8v184zm192 72c0 4.4-3.6 8-8 8h-80c-4.4 0-8-3.6-8-8V280c0-4.4 3.6-8 8-8h80c4.4 0 8 3.6 8 8v256z"></path>
                            </svg>
                        </i>
                    </div>
                    <div class="search-result-content">
                        <div class="search-result-title">${result.highlight}</div>
                        <div class="search-result-subtitle">${result.protocol} | 端口: ${result.port}</div>
                    </div>
                `;
                
                // 点击跳转到入站详情
                resultItem.addEventListener('click', () => {
                    this.hideSearch();
                    window.location.href = `${basePath}xray/inbounds?id=${result.id}`;
                });
            } else if (result.type === 'client') {
                resultItem.innerHTML = `
                    <div class="search-result-icon">
                        <i class="anticon">
                            <svg viewBox="64 64 896 896" focusable="false" width="1em" height="1em" fill="currentColor" aria-hidden="true">
                                <path d="M858.5 763.6a374 374 0 00-80.6-119.5 375.63 375.63 0 00-119.5-80.6c-.4-.2-.8-.3-1.2-.5C719.5 518 760 444.7 760 362c0-137-111-248-248-248S264 225 264 362c0 82.7 40.5 156 102.8 201.1-.4.2-.8.3-1.2.5-44.8 18.9-85 46-119.5 80.6a375.63 375.63 0 00-80.6 119.5A371.7 371.7 0 00136 901.8a8 8 0 008 8.2h60c4.4 0 7.9-3.5 8-7.8 2-77.2 33-149.5 87.8-204.3 56.7-56.7 132-87.9 212.2-87.9s155.5 31.2 212.2 87.9C779 752.7 810 825 812 902.2c.1 4.4 3.6 7.8 8 7.8h60a8 8 0 008-8.2c-1-47.8-10.9-94.3-29.5-138.2zM512 534c-45.9 0-89.1-17.9-121.6-50.4S340 407.9 340 362c0-45.9 17.9-89.1 50.4-121.6S466.1 190 512 190s89.1 17.9 121.6 50.4S684 316.1 684 362c0 45.9-17.9 89.1-50.4 121.6S557.9 534 512 534z"></path>
                            </svg>
                        </i>
                    </div>
                    <div class="search-result-content">
                        <div class="search-result-title">${result.highlight}</div>
                        <div class="search-result-subtitle">入站: ${result.inboundRemark}</div>
                    </div>
                `;
                
                // 点击跳转到入站详情
                resultItem.addEventListener('click', () => {
                    this.hideSearch();
                    window.location.href = `${basePath}xray/inbounds?id=${result.inboundId}`;
                });
            }
            
            resultsList.appendChild(resultItem);
        });
        
        resultsContainer.appendChild(resultsList);
    }

    // 清除搜索结果
    clearResults() {
        const resultsContainer = document.getElementById('search-results');
        resultsContainer.innerHTML = '';
    }
}

// 页面加载完成后初始化搜索管理器
document.addEventListener('DOMContentLoaded', () => {
    window.searchManager = new SearchManager();
}); 