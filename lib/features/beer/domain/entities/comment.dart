import 'package:freezed_annotation/freezed_annotation.dart';

part 'comment.freezed.dart';

@freezed
class Comment with _$Comment {
  const factory Comment({
    required int id,
    required int beerId,
    required int userId,
    required String text,
    required int likesCount,
    List<int>? likedBy,
    String? userName,
    DateTime? createdAt,
    @Default(false) bool isBlocked,
    @Default(false) bool needsReview,
    bool? positive,
  }) = _Comment;
}
