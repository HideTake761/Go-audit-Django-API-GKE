import logging

logger = logging.getLogger(__name__)
# logger:Python標準ライブラリであるloggingモジュールによって作られるLoggerクラスのインスタンス
# __name__:logging.getLogger(__name__):そのファイル(モジュール)のパスがロガー名になる

class LoggingMiddleware:
    # 1.サーバー起動時に一度だけ呼ばれる
    def __init__(self, get_response):
        self.get_response = get_response
    # get_response:次の処理(次のミドルウェア、最終的にはViews.py)を呼び出す、バトンを渡すためのもの。
    # 役割は関数、文法的にはインスタンス。
    # Djangoがサーバーを起動するとき、settings.pyで設定したMIDDLEWAREのチェイン(連なり)を構築。チェインの後はViews.py
    # 各ミドルウェアに「次に呼び出すべき処理」として自動的にget_responseを渡してくれる。
    # HTTPリクエストが来ると、settings.pyで設定したMIDDLEWAREを順に処理し、その後Views.pyに渡される。   

    # 2.HTTPリクエストが来るたびに呼ばれる
    def __call__(self, request): # __call__:インスタンスを関数のように呼び出すことができる
    # request:文法的にはHttpRequestクラスのインスタンス。
    # ユーザーからHTTPリクエストが来ると、まずDjango自身がその情報(メソッド、パス、ヘッダー、ユーザー情報など)
    # を元にHttpRequestオブジェクト(これがrequestの正体)を生成。
    # requestはMIDDLEWAREを順に渡され、途中で中身が変わることもある。最終的にはViews.pyに渡される。    

        # View.pyに行く前の処理。LoggingMiddlewareをsetttings.pyでMIDDLEWAREの最後に書いているので
        # このあとはViews.py    
        logger.debug(f"[REQUEST] {request.method} {request.path}")

        # 3.保持しておいた「次の処理」を呼び出す。
        # これにより、バトンが次のミドルウェア、そして最終的にViews.pyに渡される。
        # Views.pyが処理を完了すると、その結果(レスポンス)が返ってくる。        
        response = self.get_response(request)

        # Views.pyから返ってきた後の処理
        logger.debug(f"[RESPONSE] {request.method} {request.path} - {response.status_code}")
        
        # 4. 最終的なレスポンスを前の層(or Django)に返す
        return response
    # logger.debug("デバッグ用のログ"),logger.info("情報ログ"),logger.warning("警告ログ")
    # logger.error("エラーログ"),logger.critical("致命的ログ")を呼び出せる
