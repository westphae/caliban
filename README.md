# caliban
Link WeatherFlow Tempest sensor to windy.com and other services

1. Settings in YML config file; use viper & cobra
2. For tokens in config file, request station metadata from Tempest API.
3. For outside stations in metadata, subscribe to ws data from Tempest API
3. Save observations to database (sqlite)
4. Push observations to windy.com API

