// Reality协议配置类
class RealityStreamSettings extends XrayCommonClass {
    constructor(
        show = false,
        dest = 'www.microsoft.com:443',
        xver = 0,
        serverNames = 'www.microsoft.com',
        privateKey = '',
        minClientVer = '',
        maxClientVer = '',
        maxTimeDiff = 0,
        shortIds = [''],
        settings = new RealityStreamSettings.Settings()
    ) {
        super();
        this.show = show;
        this.dest = dest;
        this.xver = xver;
        this.serverNames = serverNames;
        this.privateKey = privateKey;
        this.minClientVer = minClientVer;
        this.maxClientVer = maxClientVer;
        this.maxTimeDiff = maxTimeDiff;
        this.shortIds = shortIds;
        this.settings = settings;
    }

    static fromJson(json = {}) {
        let settings;
        if (json.settings) {
            settings = RealityStreamSettings.Settings.fromJson(json.settings);
        } else {
            settings = new RealityStreamSettings.Settings();
        }

        return new RealityStreamSettings(
            json.show,
            json.dest,
            json.xver,
            json.serverNames,
            json.privateKey,
            json.minClientVer,
            json.maxClientVer,
            json.maxTimeDiff,
            json.shortIds,
            settings
        );
    }

    toJson() {
        return {
            show: this.show,
            dest: this.dest,
            xver: this.xver,
            serverNames: this.serverNames,
            privateKey: this.privateKey,
            minClientVer: this.minClientVer,
            maxClientVer: this.maxClientVer,
            maxTimeDiff: this.maxTimeDiff,
            shortIds: this.shortIds,
            settings: this.settings.toJson(),
        };
    }

    // 生成一对Reality密钥
    generateKeyPair() {
        // 向后端请求生成密钥对
        return new Promise((resolve, reject) => {
            HttpUtil.post('/xray/generateRealityKeyPair')
                .then(response => {
                    if (response.success) {
                        const keyPair = response.obj;
                        this.privateKey = keyPair.privateKey;
                        this.settings.publicKey = keyPair.publicKey;
                        resolve(keyPair);
                    } else {
                        reject(response.msg);
                    }
                })
                .catch(error => {
                    reject(error);
                });
        });
    }

    // 随机生成服务器名称
    generateRandomServerNames() {
        const popularDomains = [
            'www.microsoft.com',
            'www.amazon.com',
            'www.apple.com',
            'www.cloudflare.com',
            'www.google.com',
            'www.office.com',
            'www.azure.com',
            'www.github.com',
            'www.netflix.com',
            'www.instagram.com'
        ];
        
        // 随机选择一个域名
        const randomIndex = Math.floor(Math.random() * popularDomains.length);
        this.serverNames = popularDomains[randomIndex];
        this.dest = this.serverNames + ':443';
        
        return this.serverNames;
    }

    // 生成随机短ID
    generateRandomShortId() {
        const shortId = RandomUtil.randomHexString(8);
        this.shortIds = [shortId];
        return shortId;
    }
}

// Reality设置类
RealityStreamSettings.Settings = class extends XrayCommonClass {
    constructor(
        publicKey = '',
        fingerprint = 'chrome',
        serverName = '',
        spiderX = ''
    ) {
        super();
        this.publicKey = publicKey;
        this.fingerprint = fingerprint;
        this.serverName = serverName;
        this.spiderX = spiderX;
    }

    static fromJson(json = {}) {
        return new RealityStreamSettings.Settings(
            json.publicKey,
            json.fingerprint,
            json.serverName,
            json.spiderX
        );
    }

    toJson() {
        return {
            publicKey: this.publicKey,
            fingerprint: this.fingerprint,
            serverName: this.serverName,
            spiderX: this.spiderX
        };
    }
}; 