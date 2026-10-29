# Prueba de sangría para Aula: si se copió bien, corre sin errores.
# Caracteres especiales: ñ á é ü < > & "comillas" 'simples' \barras\


def clasificar(numeros):
    """Devuelve (pares, impares) conservando el orden."""
    pares, impares = [], []
    for n in numeros:
        if n % 2 == 0:
            pares.append(n)
        else:
            impares.append(n)
    return pares, impares


class Contador:
    def __init__(self):
        self.total = 0

    def sumar(self, valores):
        for v in valores:
            if v > 0:
                self.total += v
        return self.total


if __name__ == "__main__":
    pares, impares = clasificar(range(1, 11))
    print("pares:", pares)
    print("impares:", impares)
    print("total:", Contador().sumar([3, -1, 4]))
    print("Si ves estas 4 líneas, la sangría llegó intacta.")
