import 'package:freezed_annotation/freezed_annotation.dart';

part 'beer.freezed.dart';

@freezed
class Beer with _$Beer {
  const factory Beer({
    required int id,
    required String name,
    required String style,
    required String brewery,
    required double averageRating,
    required int ratingsCount,
    String? imageUrl,
    String? description,
    double? alcohol,
    String? taste,
    String? aroma,
    String? color,
    String? body,
    String? carbonation,
    String? finish,
    DateTime? createdAt,
    @Default(false) bool isFeatured,
  }) = _Beer;
}
